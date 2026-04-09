import OSS from 'ali-oss';

const pageTemplate = ({ novels }) => `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>小说展示</title>
  <style>
    body{font-family:Arial,"Microsoft YaHei";background:#f8fafc;margin:0;color:#1e293b}
    header{padding:18px 24px;background:#4338ca;color:#fff}
    main{max-width:1000px;margin:20px auto;padding:0 12px}
    .card{background:#fff;border-radius:10px;padding:12px 16px;margin-bottom:12px;box-shadow:0 4px 16px rgba(0,0,0,.06)}
    .meta{color:#64748b;font-size:13px}
    .row{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap}
    a{color:#4f46e5;text-decoration:none}
  </style>
</head>
<body>
  <header>
    <h1>📚 小说展示页</h1>
    <div><a style="color:#c7d2fe" href="/index.html">返回管理首页</a></div>
  </header>
  <main>
    ${novels.length > 0
    ? novels
      .map(
        (novel) => `
      <div class="card">
        <div class="row"><h3>${escapeHtml(novel.name || '')}</h3><strong>${statusLabel(novel.status)}</strong></div>
        <div class="meta">平台：${escapeHtml(novel.platform || '')} | 评分：${Number.isFinite(novel.rating) ? novel.rating : 0}/10</div>
        <div class="meta">文件：${escapeHtml(novel.file || '')}</div>
        <div>
          ${Number.isFinite(Number(novel.id))
            ? `<a href="/novels/${Number(novel.id)}.html">详情页（预留）</a>`
            : '<span class="meta">暂无详情页 ID</span>'}
          ${novel.url ? ` | <a href="${escapeHtml(novel.url)}" target="_blank">阅读链接</a>` : ''}
        </div>
      </div>`,
      )
      .join('')
    : '<div class="card">暂无小说数据</div>'}
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

  const novels = Array.isArray(payload?.novels) ? payload.novels : [];
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
