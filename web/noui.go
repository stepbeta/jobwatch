//go:build !ui

package web

import "net/http"

const noUIPage = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>JobWatch</title></head>
<body style="font: 15px system-ui, sans-serif; max-width: 640px; margin: 48px auto; line-height: 1.5">
<h1>JobWatch API is running</h1>
<p>The web UI was not embedded in this binary.</p>
<p>Build it and embed it:</p>
<pre style="background:#eee;padding:12px;border-radius:6px">cd web
npm ci
npm run build
cd ..
go build -tags ui -o jobwatch ./cmd/jobwatch</pre>
<p>For development, run <code>npm run dev</code> in <code>web/</code> and open
<a href="http://127.0.0.1:5173">http://127.0.0.1:5173</a>; Vite proxies
<code>/api</code> to this server.</p>
<p>The API health endpoint is <a href="/api/health">/api/health</a>.</p>
</body>
</html>
`

// Handler returns a placeholder page explaining how to build the UI.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(noUIPage))
	})
}
