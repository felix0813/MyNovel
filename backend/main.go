package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Novel struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Platform    string    `json:"platform"`
	URL         string    `json:"url"`
	File        string    `json:"file"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Rating      int       `json:"rating"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type NovelInput struct {
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	URL         string `json:"url"`
	File        string `json:"file"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Rating      int    `json:"rating"`
}

type app struct {
	db       *pgxpool.Pool
	exporter *ossExporter
}

type ossExporter struct {
	enabled    bool
	bucket     *oss.Bucket
	objectName string
}

type syncPayload struct {
	GeneratedAt time.Time `json:"generated_at"`
	Novel       Novel     `json:"novel"`
}

func main() {
	ctx := context.Background()
	dsn := env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mynovel?sslmode=disable")
	log.Printf("initializing backend service")
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect db failed: %v", err)
	}
	defer db.Close()
	log.Printf("database pool initialized")

	exporter, err := newOSSExporter()
	if err != nil {
		log.Fatalf("init oss exporter failed: %v", err)
	}

	a := &app{db: db, exporter: exporter}
	mux := http.NewServeMux()
	mux.HandleFunc("/myNovel/healthz", a.handleHealth)
	mux.HandleFunc("/myNovel/api/novels", a.handleNovels)
	mux.HandleFunc("/myNovel/api/novels/", a.handleNovelByID)

	addr := env("ADDR", ":8080")
	log.Printf("backend listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(withLogging(mux))); err != nil {
		log.Fatal(err)
	}
}

func newOSSExporter() (*ossExporter, error) {
	endpoint := os.Getenv("OSS_ENDPOINT")
	ak := os.Getenv("OSS_ACCESS_KEY_ID")
	sk := os.Getenv("OSS_ACCESS_KEY_SECRET")
	bucketName := env("OSS_JSON_BUCKET", "novel-json")
	objectName := env("OSS_JSON_OBJECT", "novels/%d.json")

	if endpoint == "" || ak == "" || sk == "" {
		log.Printf("OSS env not fully configured, skip json upload")
		return &ossExporter{enabled: false}, nil
	}

	// 创建带有 2 秒超时的 OSS 客户端
	client, err := oss.New(endpoint, ak, sk,
		oss.Timeout(2, 10)) // 重试次数设为 1
	if err != nil {
		return nil, err
	}

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return nil, err
	}
	log.Printf("OSS exporter enabled, bucket=%s objectPattern=%s", bucketName, objectName)

	return &ossExporter{enabled: true, bucket: bucket, objectName: objectName}, nil
}

func (e *ossExporter) upload(ctx context.Context, novels []Novel) error {
	if !e.enabled {
		log.Printf("skip OSS upload because exporter is disabled")
		return nil
	}
	log.Printf("start OSS upload, novels_count=%d", len(novels))
	for _, novel := range novels {
		payload := syncPayload{GeneratedAt: time.Now().UTC(), Novel: novel}
		b, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			log.Printf("marshal novel payload failed, novel_id=%d err=%v", novel.ID, err)
			return err
		}
		reader := strings.NewReader(string(b))
		objectName := fmt.Sprintf(e.objectName, novel.ID)
		if err := e.bucket.PutObject(objectName, reader, oss.ContentType("application/json")); err != nil {
			log.Printf("upload novel json failed, novel_id=%d object=%s err=%v", novel.ID, objectName, err)
			return err
		}
		log.Printf("uploaded novel json, novel_id=%d object=%s", novel.ID, objectName)
	}
	log.Printf("OSS upload completed, novels_count=%d", len(novels))
	return nil
}

func (a *app) handleHealth(w http.ResponseWriter, r *http.Request) {
	// 检查数据库连接
	dbStatus := "ok"
	if a.db.Ping(r.Context()) != nil {
		dbStatus = "failed"
	}

	// 检查 OSS 连接
	ossStatus := "disabled"
	if a.exporter.enabled {
		// 尝试列出 bucket 中的对象来验证连接
		_, err := a.exporter.bucket.ListObjects(oss.MaxKeys(1))
		if err != nil {
			ossStatus = "failed"
		} else {
			ossStatus = "ok"
		}
	}

	healthInfo := map[string]string{
		"status":     "ok",
		"database":   dbStatus,
		"oss_export": ossStatus,
	}

	// 如果有任何组件失败，整体状态应为失败
	if dbStatus == "failed" || ossStatus == "failed" {
		healthInfo["status"] = "failed"
		if dbStatus == "failed" {
			log.Printf("Health check failed: database connection issue")
		}
		if ossStatus == "failed" {
			log.Printf("Health check failed: OSS connection issue")
		}
		writeJSON(w, http.StatusServiceUnavailable, healthInfo)
		return
	}

	writeJSON(w, http.StatusOK, healthInfo)
}

func (a *app) handleNovels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listNovels(w, r)
	case http.MethodPost:
		a.createNovel(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *app) handleNovelByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid novel id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.getNovel(w, r, id)
	case http.MethodPut:
		a.updateNovel(w, r, id)
	case http.MethodDelete:
		a.deleteNovel(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *app) listNovels(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	log.Printf("list novels requested, q=%q status=%q", q, status)

	query := `
		SELECT id, name, platform, url, file_path, description, status::text, rating, created_at, updated_at
		FROM novels
		WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR platform ILIKE '%' || $1 || '%')
		AND ($2 = '' OR status::text = $2)
		ORDER BY updated_at DESC`
	rows, err := a.db.Query(r.Context(), query, q, status)
	if err != nil {
		log.Printf("list novels query failed, q=%q status=%q err=%v", q, status, err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	novels := make([]Novel, 0)
	for rows.Next() {
		var n Novel
		if err := rows.Scan(&n.ID, &n.Name, &n.Platform, &n.URL, &n.File, &n.Description, &n.Status, &n.Rating, &n.CreatedAt, &n.UpdatedAt); err != nil {
			log.Printf("list novels scan failed err=%v", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		novels = append(novels, n)
	}
	log.Printf("list novels completed, count=%d", len(novels))
	writeJSON(w, http.StatusOK, novels)
}

func (a *app) getNovel(w http.ResponseWriter, r *http.Request, id int64) {
	log.Printf("get novel requested, id=%d", id)
	var n Novel
	err := a.db.QueryRow(r.Context(), `
		SELECT id, name, platform, url, file_path, description, status::text, rating, created_at, updated_at
		FROM novels WHERE id=$1`, id).Scan(&n.ID, &n.Name, &n.Platform, &n.URL, &n.File, &n.Description, &n.Status, &n.Rating, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("get novel not found, id=%d", id)
			writeError(w, http.StatusNotFound, "novel not found")
			return
		}
		log.Printf("get novel query failed, id=%d err=%v", id, err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("get novel completed, id=%d", id)
	writeJSON(w, http.StatusOK, n)
}

func (a *app) createNovel(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeAndValidateNovel(w, r)
	if !ok {
		log.Printf("create novel validation failed")
		return
	}
	log.Printf("create novel requested, name=%q platform=%q status=%q rating=%d", in.Name, in.Platform, in.Status, in.Rating)
	var n Novel
	err := a.db.QueryRow(r.Context(), `
		INSERT INTO novels(name, platform, url, file_path, description, status, rating)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, name, platform, url, file_path, description, status::text, rating, created_at, updated_at`,
		in.Name, in.Platform, in.URL, in.File, in.Description, in.Status, in.Rating,
	).Scan(&n.ID, &n.Name, &n.Platform, &n.URL, &n.File, &n.Description, &n.Status, &n.Rating, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		log.Printf("create novel db insert failed, name=%q err=%v", in.Name, err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("create novel succeeded, id=%d", n.ID)
	if err := a.syncNovels(r.Context()); err != nil {
		log.Printf("create novel sync failed, id=%d err=%v", n.ID, err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("novel created but sync failed: %v", err))
		return
	}
	log.Printf("create novel completed, id=%d", n.ID)
	writeJSON(w, http.StatusCreated, n)
}

func (a *app) updateNovel(w http.ResponseWriter, r *http.Request, id int64) {
	in, ok := decodeAndValidateNovel(w, r)
	if !ok {
		log.Printf("update novel validation failed, id=%d", id)
		return
	}
	log.Printf("update novel requested, id=%d status=%q rating=%d", id, in.Status, in.Rating)
	var n Novel
	err := a.db.QueryRow(r.Context(), `
		UPDATE novels
		SET name=$1, platform=$2, url=$3, file_path=$4, description=$5, status=$6, rating=$7, updated_at=NOW()
		WHERE id=$8
		RETURNING id, name, platform, url, file_path, description, status::text, rating, created_at, updated_at`,
		in.Name, in.Platform, in.URL, in.File, in.Description, in.Status, in.Rating, id,
	).Scan(&n.ID, &n.Name, &n.Platform, &n.URL, &n.File, &n.Description, &n.Status, &n.Rating, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("update novel not found, id=%d", id)
			writeError(w, http.StatusNotFound, "novel not found")
			return
		}
		log.Printf("update novel db update failed, id=%d err=%v", id, err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("update novel succeeded, id=%d", id)
	if err := a.syncNovels(r.Context()); err != nil {
		log.Printf("update novel sync failed, id=%d err=%v", id, err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("novel updated but sync failed: %v", err))
		return
	}
	log.Printf("update novel completed, id=%d", id)
	writeJSON(w, http.StatusOK, n)
}

func (a *app) deleteNovel(w http.ResponseWriter, r *http.Request, id int64) {
	log.Printf("delete novel requested, id=%d", id)
	res, err := a.db.Exec(r.Context(), `DELETE FROM novels WHERE id=$1`, id)
	if err != nil {
		log.Printf("delete novel db delete failed, id=%d err=%v", id, err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if res.RowsAffected() == 0 {
		log.Printf("delete novel not found, id=%d", id)
		writeError(w, http.StatusNotFound, "novel not found")
		return
	}
	log.Printf("delete novel succeeded, id=%d", id)
	if err := a.syncNovels(r.Context()); err != nil {
		log.Printf("delete novel sync failed, id=%d err=%v", id, err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("novel deleted but sync failed: %v", err))
		return
	}
	log.Printf("delete novel completed, id=%d", id)
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (a *app) syncNovels(ctx context.Context) error {
	log.Printf("start syncing novels to OSS")
	rows, err := a.db.Query(ctx, `
		SELECT id, name, platform, url, file_path, description, status::text, rating, created_at, updated_at
		FROM novels ORDER BY updated_at DESC`)
	if err != nil {
		log.Printf("sync novels query failed err=%v", err)
		return err
	}
	defer rows.Close()

	novels := make([]Novel, 0)
	for rows.Next() {
		var n Novel
		if err := rows.Scan(&n.ID, &n.Name, &n.Platform, &n.URL, &n.File, &n.Description, &n.Status, &n.Rating, &n.CreatedAt, &n.UpdatedAt); err != nil {
			log.Printf("sync novels scan failed err=%v", err)
			return err
		}
		novels = append(novels, n)
	}
	if err := a.exporter.upload(ctx, novels); err != nil {
		log.Printf("sync novels upload failed, count=%d err=%v", len(novels), err)
		return err
	}
	log.Printf("sync novels completed, count=%d", len(novels))
	return nil
}

func decodeAndValidateNovel(w http.ResponseWriter, r *http.Request) (NovelInput, bool) {
	var in NovelInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return in, false
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Platform = strings.TrimSpace(in.Platform)
	in.URL = strings.TrimSpace(in.URL)
	in.File = strings.TrimSpace(in.File)
	in.Description = strings.TrimSpace(in.Description)
	in.Status = strings.TrimSpace(in.Status)

	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return in, false
	}
	switch in.Status {
	case "unread", "reading", "finished":
	default:
		writeError(w, http.StatusBadRequest, "status must be one of unread/reading/finished")
		return in, false
	}
	if in.Rating < 0 || in.Rating > 10 {
		writeError(w, http.StatusBadRequest, "rating must be between 0 and 10")
		return in, false
	}
	return in, true
}

func parseID(path string) (int64, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return 0, errors.New("bad path")
	}
	return strconv.ParseInt(parts[len(parts)-1], 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func withCORS(next http.Handler) http.Handler {
	allowedOrigin := env("CORS_ALLOW_ORIGIN", "*")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		log.Printf("[%s] %s %s - Status: %d - Duration: %v",
			r.Method,
			r.RemoteAddr,
			r.URL.Path,
			wrapped.statusCode,
			duration,
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
