// api.js — POST + JSON 的 RPC helper.
// 路径与后端 server 注册一致 (main.go: http.HandleFunc("/api/GetPostList", ...)).

export async function postJson(path, data, timeoutMs) {
  const controller = new AbortController()
  const timer = timeoutMs ? setTimeout(() => controller.abort(), timeoutMs) : null
  try {
    const resp = await fetch('/api' + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data || {}),
      signal: controller.signal,
    })
    if (!resp.ok) throw new Error('HTTP ' + resp.status)
    return resp.json()
  } finally {
    if (timer) clearTimeout(timer)
  }
}
