import OSS from 'ali-oss';

const pageTemplate = ({ novel }) => `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <link rel="icon" href="/favicon.svg" type="image/svg+xml" />
  <title>${escapeHtml(novel?.name || '小说详情')}</title>
  <style>
    :root {
      --bg: #f6f8fc;
      --card: #fff;
      --text: #1f2937;
      --muted: #6b7280;
      --primary: #4f46e5;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
      background: var(--bg);
      color: var(--text);
    }
    .topbar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 1rem 2rem;
      background: linear-gradient(135deg, #4338ca, #6366f1);
      color: #fff;
      gap: 12px;
      flex-wrap: wrap;
    }
    .topbar h1 {
      margin: 0;
      font-size: 1.6rem;
    }
    .topbar .badge {
      border-radius: 999px;
      padding: 0.3rem 0.8rem;
      background: rgba(255, 255, 255, 0.16);
      font-size: 0.9rem;
    }
    .container {
      max-width: 1080px;
      margin: 1.5rem auto;
      padding: 0 1rem;
    }
    .card {
      background: var(--card);
      border-radius: 12px;
      padding: 1rem;
      box-shadow: 0 6px 20px rgba(79, 70, 229, 0.08);
      margin-bottom: 1rem;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.9rem;
    }
    .item {
      padding: 0.85rem;
      border: 1px solid #edf2ff;
      border-radius: 10px;
      background: #fafbff;
      min-height: 88px;
    }
    .item h2 {
      margin: 0 0 0.45rem;
      color: var(--muted);
      font-size: 0.9rem;
      font-weight: 500;
    }
    .item p {
      margin: 0;
      line-height: 1.65;
      white-space: pre-wrap;
      word-break: break-word;
    }
    .item.full { grid-column: 1 / -1; }
    a { color: var(--primary); text-decoration: none; }
    a:hover { text-decoration: underline; }
    .actions {
      display: flex;
      gap: 0.75rem;
      flex-wrap: wrap;
      margin-top: 0.25rem;
    }
    @media (max-width: 720px) {
      .topbar { padding: 1rem; }
      .grid { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <header class="topbar">
    <h1>📚 ${escapeHtml(novel?.name || '小说详情')}</h1>
    <span class="badge">${statusLabel(novel?.status)}</span>
  </header>

  <main class="container">
    <section class="card grid">
      <article class="item">
        <h2>平台</h2>
        <p>${escapeHtml(novel?.platform || '-')}</p>
      </article>
      <article class="item">
        <h2>评分</h2>
        <p>${normalizeRating(novel?.rating)}/10</p>
      </article>
      <article class="item">
        <h2>文件</h2>
        <p>${escapeHtml(novel?.file || '-')}</p>
      </article>
      <article class="item">
        <h2>ID</h2>
        <p>${Number.isFinite(Number(novel?.id)) ? Number(novel?.id) : '-'}</p>
      </article>
      <article class="item full">
        <h2>简介</h2>
        <p>${escapeHtml(novel?.description || '暂无简介')}</p>
      </article>
      <article class="item full">
        <h2>操作</h2>
        <div class="actions">
          ${Number.isFinite(Number(novel?.id))
            ? `<a href="https://novel.wzfly.top/edit.html?id=${Number(novel?.id)}">前往编辑页</a>`
            : '<span>暂无可用 ID</span>'}
          ${novel?.url ? `<a href="${escapeHtml(novel.url)}" target="_blank" rel="noopener noreferrer">阅读链接</a>` : ''}
        </div>
      </article>
    </section>
  </main>
</body>
</html>`;

function normalizeRating (rating) {
  const n = Number(rating);
  return Number.isFinite(n) ? n : 0;
}

function statusLabel (status) {
  if (status === 'unread') return '未读';
  if (status === 'reading') return '在读';
  return '已读';
}

function escapeHtml (raw) {
  return String(raw)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function parseEvent (event) {
  if (typeof event === 'string') {
    return JSON.parse(event);
  }
  if (Buffer.isBuffer(event)) {
    return JSON.parse(event.toString('utf8'));
  }
  return event;
}

function createClient () {
  const endpoint = process.env.OSS_ENDPOINT;
  const accessKeyId = process.env.OSS_ACCESS_KEY_ID;
  const accessKeySecret = process.env.OSS_ACCESS_KEY_SECRET;

  if (!endpoint || !accessKeyId || !accessKeySecret) {
    throw new Error('missing OSS endpoint or credentials');
  }

  return new OSS({
    endpoint,
    accessKeyId,
    accessKeySecret,
    bucket: process.env.TARGET_HTML_BUCKET || 'novels-html',
  });
}

export const handler = async (event) => {
  console.log('[handler] received event', {
    eventType: typeof event,
    isBuffer: Buffer.isBuffer(event),
  });

  const parsed = parseEvent(event);
  if (!parsed?.events?.length) {
    throw new Error('event is empty');
  }

  const sourceBucket = parsed.events[0]?.oss?.bucket?.name;
  const sourceObjectKey = parsed.events[0]?.oss?.object?.key;
  if (!sourceBucket || !sourceObjectKey) {
    throw new Error('missing OSS event bucket/object');
  }
  console.log('[handler] parsed OSS trigger', { sourceBucket, sourceObjectKey });

  const targetPrefix = (process.env.TARGET_HTML_PREFIX || 'novels').replace(/^\/+|\/+$/g, '');
  const targetKey = `${targetPrefix}/${sourceObjectKey.replace('.json', '.html').split('/').pop()}`;
  console.log('[handler] target key resolved', { targetKey });

  const sourceClient = createClient();
  sourceClient.options.bucket = sourceBucket;
  console.log('[handler] fetching source object');

  const sourceResp = await sourceClient.get(sourceObjectKey);
  const payload = typeof sourceResp.content === 'string'
    ? JSON.parse(sourceResp.content)
    : JSON.parse(sourceResp.content.toString('utf8'));
  console.log('[handler] source payload loaded');

  const novel = payload?.novel ?? payload;
  if (!novel || typeof novel !== 'object' || Array.isArray(novel)) {
    throw new Error('payload must contain one novel object');
  }
  const normalizedNovel = {
    ...novel,
    id: Number(novel?.id),
    rating: normalizeRating(novel?.rating),
  };
  console.log('[handler] novel normalized', { id: normalizedNovel.id, name: normalizedNovel.name });

  const page = pageTemplate({ novel: normalizedNovel });

  const targetClient = createClient();
  console.log('[handler] uploading generated html');
  await targetClient.put(targetKey, Buffer.from(page, 'utf8'), {
    headers: { 'Content-Type': 'text/html; charset=utf-8' },
  });
  console.log('[handler] upload completed', { targetKey });

  return `generated ${targetKey}`;
};
