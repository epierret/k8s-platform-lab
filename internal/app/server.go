package app

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.root)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", s.readyz)   // 503 tant que s.ready == false
	mux.HandleFunc("GET /version", s.version) // BuildInfo injecté par ldflags
	return s.logRequests(mux)                  // slog JSON : method, path, status, durée
}
