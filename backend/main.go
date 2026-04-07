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
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect db failed: %v", err)
	}
	defer db.Close()

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
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
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

	client, err := oss.New(endpoint, ak, sk)
	if err != nil {
		return nil, err
	}
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return nil, err
	}
	return &ossExporter{enabled: true, bucket: bucket, objectName: objectName}, nil
}

func (e *ossExporter) upload(ctx context.Context, novels []Novel) error {
	if !e.enabled {
		return nil
	}
	for _, novel := range novels {
		payload := syncPayload{GeneratedAt: time.Now().UTC(), Novel: novel}
		b, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		reader := strings.NewReader(string(b))
		objectName := fmt.Sprintf(e.objectName, novel.ID)
		if err := e.bucket.PutObject(objectName, reader, oss.ContentType("application/json")); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

	query := `
		SELECT id, name, platform, url, file_path, description, status::text, rating, created_at, updated_at
		FROM novels
		WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR platform ILIKE '%' || $1 || '%')
		AND ($2 = '' OR status::text = $2)
		ORDER BY updated_at DESC`
	rows, err := a.db.Query(r.Context(), query, q, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	novels := make([]Novel, 0)
	for rows.Next() {
		var n Novel
		if err := rows.Scan(&n.ID, &n.Name, &n.Platform, &n.URL, &n.File, &n.Description, &n.Status, &n.Rating, &n.CreatedAt, &n.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		novels = append(novels, n)
	}
	writeJSON(w, http.StatusOK, novels)
}

func (a *app) getNovel(w http.ResponseWriter, r *http.Request, id int64) {
	var n Novel
	err := a.db.QueryRow(r.Context(), `
		SELECT id, name, platform, url, file_path, description, status::text, rating, created_at, updated_at
		FROM novels WHERE id=$1`, id).Scan(&n.ID, &n.Name, &n.Platform, &n.URL, &n.File, &n.Description, &n.Status, &n.Rating, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "novel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (a *app) createNovel(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeAndValidateNovel(w, r)
	if !ok {
		return
	}
	var n Novel
	err := a.db.QueryRow(r.Context(), `
		INSERT INTO novels(name, platform, url, file_path, description, status, rating)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, name, platform, url, file_path, description, status::text, rating, created_at, updated_at`,
		in.Name, in.Platform, in.URL, in.File, in.Description, in.Status, in.Rating,
	).Scan(&n.ID, &n.Name, &n.Platform, &n.URL, &n.File, &n.Description, &n.Status, &n.Rating, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.syncNovels(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("novel created but sync failed: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (a *app) updateNovel(w http.ResponseWriter, r *http.Request, id int64) {
	in, ok := decodeAndValidateNovel(w, r)
	if !ok {
		return
	}
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
			writeError(w, http.StatusNotFound, "novel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.syncNovels(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("novel updated but sync failed: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (a *app) deleteNovel(w http.ResponseWriter, r *http.Request, id int64) {
	res, err := a.db.Exec(r.Context(), `DELETE FROM novels WHERE id=$1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "novel not found")
		return
	}
	if err := a.syncNovels(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("novel deleted but sync failed: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (a *app) syncNovels(ctx context.Context) error {
	rows, err := a.db.Query(ctx, `
		SELECT id, name, platform, url, file_path, description, status::text, rating, created_at, updated_at
		FROM novels ORDER BY updated_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	novels := make([]Novel, 0)
	for rows.Next() {
		var n Novel
		if err := rows.Scan(&n.ID, &n.Name, &n.Platform, &n.URL, &n.File, &n.Description, &n.Status, &n.Rating, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return err
		}
		novels = append(novels, n)
	}
	return a.exporter.upload(ctx, novels)
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
	if len(parts) < 3 {
		return 0, errors.New("bad path")
	}
	return strconv.ParseInt(parts[2], 10, 64)
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

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
