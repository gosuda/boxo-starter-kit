package trustless

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ipfs/boxo/gateway"

	"github.com/gosuda/boxo-starter-kit/pkg/security"
)

type GatewayWrapper struct {
	port     int
	Server   *http.Server
	security *security.SecurityMiddleware
}

func NewGatewayWrapper(port int, urls []string) (*GatewayWrapper, error) {
	var err error
	if port == 0 {
		port = 8080
	}

	// Set up security middleware
	securityMiddleware := security.NewSecurityMiddleware(security.DefaultSecurityConfig())

	gatewayWrapper := &GatewayWrapper{
		port:     port,
		security: securityMiddleware,
	}

	fetcher, err := gateway.NewRemoteCarFetcher(urls, nil)
	if err != nil {
		return nil, err
	}
	fetcher, err = gateway.NewRetryCarFetcher(fetcher, 3)
	if err != nil {
		return nil, err
	}
	backend, err := gateway.NewCarBackend(fetcher)
	if err != nil {
		return nil, err
	}
	handler := gateway.NewHandler(gateway.Config{}, backend)
	mux := http.NewServeMux()
	mux.HandleFunc("/", gatewayWrapper.handleRoot)
	mux.Handle("/ipfs/", handler)
	mux.Handle("/ipns/", handler)

	// Apply security middleware to the entire mux
	secureHandler := gatewayWrapper.security.Handler()(mux)

	gatewayWrapper.Server = &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		Handler:        secureHandler,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	return gatewayWrapper, nil
}

func (g *GatewayWrapper) Start() error {
	return g.Server.ListenAndServe()
}

func (g *GatewayWrapper) Close() error {
	if g.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return g.Server.Shutdown(ctx)
	}
	return nil
}

func (g *GatewayWrapper) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <title>Trustless IPFS Gateway</title>
  <style>
    :root {
      --bg: #0b0f14; --fg: #e6edf3; --muted: #98a2b3; --card:#0f1620; --accent:#4f46e5;
      --ok:#10b981; --warn:#f59e0b; --err:#ef4444; --border:#1f2937; --code:#0b1220;
    }
    @media (prefers-color-scheme: light) {
      :root { --bg:#ffffff; --fg:#0f172a; --muted:#475569; --card:#f8fafc; --accent:#4f46e5; --border:#e2e8f0; --code:#f1f5f9; }
    }
    * { box-sizing: border-box; }
    body { background: var(--bg); color: var(--fg); font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, "Noto Sans", Helvetica, Arial, "Apple Color Emoji", "Segoe UI Emoji"; margin:0; }
    .wrap { max-width: 1200px; margin: 48px auto; padding: 0 0px; }
    .hero { text-align:center; margin-bottom: 28px; }
    .hero h1 { font-size: 32px; margin: 0 0 8px; }
    .sub { color: var(--muted); font-size: 15px; }

    .grid {
      display: flex;
      flex-direction: column;
      gap: 16px;
      width: max-content;
      margin: 0 auto;
    }
    .card {
      width: 100%%;
      max-width: 960px;
      background: var(--card);
      border:1px solid var(--border);
      border-radius: 14px;
      padding: 16px 18px;
    }

    h2 { font-size: 18px; margin: 0 0 8px; display:flex; align-items:center; gap:8px;}
    p { margin: 8px 0 12px; color: var(--muted); }
    .code { background: var(--code); border:1px solid var(--border); padding: 10px 12px; border-radius: 10px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; font-size: 13px; overflow:auto; }
    .list { padding-left: 18px; margin: 8px 0 0; color: var(--muted); }
    .k { color: var(--fg); }
    a { color: var(--accent); text-decoration: none; }
    a:hover { text-decoration: underline; }
    .badge { display:inline-flex; align-items:center; gap:6px; border-radius: 999px; padding: 4px 10px; font-size: 12px; border:1px solid var(--border); background: rgba(79,70,229,.08); }
    .status { font-weight:600; }
    .ok { color: var(--ok); }
    .warn { color: var(--warn); }
    .err { color: var(--err); }
    .muted { color: var(--muted); }
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; }
    .footer { margin-top: 28px; color: var(--muted); font-size: 12px; text-align:center; }
    .pill { display:inline-block; padding:2px 8px; border-radius: 999px; border:1px solid var(--border); background: var(--card); font-size: 12px; color: var(--muted); }
    .soon { opacity:.7; }

    /* Improved input/button styles */
    .row { display:flex; gap:8px; width:100%%; max-width:640px; margin-top:8px; }
    .input {
      flex:1; padding:12px 14px; border-radius:12px; border:1px solid var(--border);
      background: var(--bg); color: var(--fg); font-size:15px;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      transition: all .2s ease;
    }
    .input::placeholder { color: var(--muted); }
    .input:focus {
      outline:none; border-color: var(--accent);
      box-shadow:0 0 0 2px rgba(79,70,229,.30);
    }
    .btn {
      padding:12px 18px; border-radius:12px; border:1px solid var(--border);
      background: var(--accent); color:#fff; font-weight:600; font-size:14px;
      cursor:pointer; transition: background .2s ease;
    }
    .btn:hover { background:#3730a3; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="hero">
      <h1>🌐 Trustless IPFS Gateway</h1>
      <div class="sub">Stateless proxy that streams CAR/blocks from upstreams · port <span class="mono">%d</span></div>
      <div style="margin-top:10px;">
        <span id="health" class="badge"><span>health</span><span class="status muted">checking…</span></span>
      </div>
    </div>

    <div class="grid">
      <div class="card">
        <h2>📖 How to use</h2>
        <p>Trustless paths fetch from upstreams and stream the result as-is.</p>
        <div class="code">GET http://localhost:%d/ipfs/&lt;CID&gt;</div>
        <div class="code" style="margin-top:8px;">GET http://localhost:%d/ipns/&lt;NAME&gt;</div>
        <p style="margin-top:10px;">Tip: many upstreams support CAR output:</p>
        <div class="code">GET http://localhost:%d/ipfs/&lt;CID&gt;?format=car</div>
      </div>

      <div class="card">
        <h2>🧪 Examples</h2>
        <p>Enter a <b>CAR URL</b> or a <b>CID</b>. We’ll redirect you to the right path.</p>
        <form class="row" onsubmit="return openCar(event)">
          <input id="carinput" class="input" type="text" spellcheck="false" autocomplete="off"
                 value="/ipfs/QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o?format=car" />
          <button class="btn" type="submit">🚀 Open</button>
        </form>
        <p class="muted" style="margin-top:8px;">
          • CID → <span class="mono">/ipfs/&lt;CID&gt;?format=car</span><br/>
          • URL ending with <span class="mono">.car</span> → open as-is
        </p>
      </div>

      <div class="card">
        <h2>🛠️ CLI</h2>
        <p>Quick curl checks:</p>
        <div class="code">curl -I http://localhost:%d/ipfs/&lt;CID&gt;</div>
        <div class="code" style="margin-top:8px;">curl -L http://localhost:%d/ipfs/&lt;CID&gt; -o out.bin</div>
        <p style="margin-top:10px;">Health:</p>
        <div class="code">curl -s http://localhost:%d/healthz</div>
      </div>

      <div class="card soon">
        <h2>🧭 Local fallback (coming soon)</h2>
        <p>If upstreams fail, we plan to serve from a local cache/IPLD:</p>
        <ul class="list">
          <li><span class="pill">Fetcher order</span> local cache → remote upstreams</li>
          <li><span class="pill">Local CAR</span> <span class="mono">/local/ipfs/&lt;CID&gt;.car</span></li>
          <li><span class="pill">Direct file</span> <span class="mono">/local/file/&lt;CID&gt;</span> (UnixFS)</li>
          <li><span class="pill">Pin/GC</span> policies &amp; metrics</li>
        </ul>
      </div>

      <div class="card">
        <h2>ℹ️ About</h2>
        <p>This gateway is <b>trustless</b> by default: it does not persist blocks.
           A separate worker may import CARs into a local store. You can then enable a
           <b>multi-fetcher</b> (local → remote) for latency and resilience.</p>
        <p class="muted">Default upstreams are typically <span class="mono">ipfs.io</span>, <span class="mono">dweb.link</span> (override with <span class="mono">--upstream</span>).</p>
      </div>
    </div>

    <div class="footer">© %d · Trustless Gateway</div>
  </div>

  <script>
    async function ping() {
      const el = document.getElementById('health');
      const st = el.querySelector('.status');
      try {
        const r = await fetch('/healthz', {cache:'no-store'});
        if (r.ok) { st.textContent = 'ok'; st.className = 'status ok'; }
        else { st.textContent = 'degraded'; st.className = 'status warn'; }
      } catch (e) {
        st.textContent = 'down'; st.className = 'status err';
      }
    }

    function looksLikeCID(s) {
      return /^Qm[1-9A-HJ-NP-Za-km-z]{44,}$/.test(s) || /^bafy[0-9a-z]{20,}$/.test(s);
    }
    function isAbsoluteURL(s) { return /^https?:\/\//i.test(s); }
    function addFormatIfMissing(path) { return path.includes('?') ? path : (path + '?format=car'); }

    function openCar(e) {
      e.preventDefault();
      let v = (document.getElementById('carinput').value || '').trim();
      if (!v) return false;

      // 1) Absolute URL → go as-is (supports https://.../*.car)
      if (isAbsoluteURL(v)) {
        window.location.href = v;
        return false;
      }
      // 2) Already /ipfs/ or /ipns/ → use as-is (avoid duplicating ?format)
      if (v.startsWith('/ipfs/') || v.startsWith('/ipns/')) {
        window.location.href = v;
        return false;
      }
      // 3) CID → /ipfs/<CID>[?format=car]
      if (looksLikeCID(v)) {
        window.location.href = addFormatIfMissing('/ipfs/' + encodeURIComponent(v));
        return false;
      }
      // 4) Ends with .car but missing scheme → ask for full URL
      if (v.toLowerCase().endsWith('.car')) {
        alert('Please enter a full CAR URL including http(s)://');
        return false;
      }
      // 5) Fallback: treat as CID-like
      window.location.href = addFormatIfMissing('/ipfs/' + encodeURIComponent(v));
      return false;
    }

    ping(); setInterval(ping, 5000);
  </script>
</body>
</html>`, g.port, g.port, g.port, g.port, g.port, g.port, g.port, time.Now().Year())
}
