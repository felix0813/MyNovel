import OSS from 'ali-oss';

const pageTemplate = ({ novels }) => `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <link rel="icon" href="/favicon.svg" type="image/svg+xml" />
  <title>小说展示</title>
  <style>
    :root{color-scheme:light only}
    *{box-sizing:border-box}
    body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"PingFang SC","Microsoft YaHei",sans-serif;background:#f4f6f8;margin:0;color:#1f2937}
    .wrap{max-width:900px;margin:0 auto;padding:36px 16px 48px}
    .header{margin-bottom:18px}
    h1{margin:0;font-size:28px;font-weight:700;letter-spacing:.3px}
    .list{display:grid;gap:10px}
    .card{background:#fff;border:1px solid #e5e7eb;border-radius:12px;padding:14px 16px}
    .row{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap}
    .name{font-size:17px;font-weight:600}
    .badge{font-size:12px;padding:2px 8px;border-radius:999px;background:#eef2ff;color:#4338ca}
    .meta{margin-top:8px;color:#6b7280;font-size:13px}
    .actions{margin-top:10px;display:flex;gap:12px;flex-wrap:wrap}
    a{color:#2563eb;text-decoration:none}
    a:hover{text-decoration:underline}
    .empty{padding:24px;text-align:center;color:#6b7280}
  </style>
</head>
<body>
  <main class="wrap">
    <header class="header">
      <h1>小说展示</h1>
    </header>
    <section class="list">
    ${novels.length > 0
    ? novels
      .map(
        (novel) => `
      <div class="card">
        <div class="row">
          <div class="name">${escapeHtml(novel.name || '')}</div>
          <span class="badge">${statusLabel(novel.status)}</span>
        </div>
        <div class="meta">平台：${escapeHtml(novel.platform || '')} | 评分：${Number.isFinite(novel.rating) ? novel.rating : 0}/10</div>
        <div class="meta">文件：${escapeHtml(novel.file || '')}</div>
        <div class="actions">
          ${Number.isFinite(Number(novel.id))
            ? `<a href="https://novel.wzfly.top/edit.html?id=${Number(novel.id)}">前往编辑页</a>`
            : '<span class="meta">暂无可用 ID</span>'}
          ${novel.url ? `<a href="${escapeHtml(novel.url)}" target="_blank" rel="noopener noreferrer">阅读链接</a>` : ''}
        </div>
      </div>`,
      )
      .join('')
    : '<div class="card empty">暂无小说数据</div>'}
    </section>
  </main>
</body>
</html>`;

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

  const novels = Array.isArray(payload?.novels)
    ? payload.novels
    : (payload?.novel ? [payload.novel] : []);
  novels.sort((a, b) => (Number(b?.rating) || 0) - (Number(a?.rating) || 0));
  const normalizedNovels = novels.map((novel) => ({
    ...novel,
    id: Number(novel?.id),
  }));
  console.log('[handler] novels normalized', { count: normalizedNovels.length });

  const page = pageTemplate({ novels: normalizedNovels });

  const targetClient = createClient();
  console.log('[handler] uploading generated html');
  await targetClient.put(targetKey, Buffer.from(page, 'utf8'), {
    headers: { 'Content-Type': 'text/html; charset=utf-8' },
  });
  console.log('[handler] upload completed', { targetKey });

  return `generated ${targetKey}`;
};
