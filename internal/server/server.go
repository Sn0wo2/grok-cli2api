package server

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Sn0wo2/grok-cli2api/internal/auth"
	"github.com/Sn0wo2/grok-cli2api/internal/config"
	"github.com/Sn0wo2/grok-cli2api/internal/proxy"
)

type Server struct {
	log   *slog.Logger
	pool  *auth.Pool
	proxy *proxy.Client
	gin   *gin.Engine
}

func New(log *slog.Logger, pool *auth.Pool) *Server {
	gin.SetMode(gin.ReleaseMode)
	s := &Server{
		log:   log,
		pool:  pool,
		proxy: proxy.NewClient(log),
	}
	s.gin = gin.New()
	s.gin.Use(gin.Recovery())

	v1 := s.gin.Group("/v1")
	v1.GET("/models", func(c *gin.Context) {
		s.proxyRequest(c, func(rec auth.AccountRecord) (*http.Response, error) {
			return s.proxy.GetModels(rec)
		}, false)
	})
	v1.POST("/responses", s.handleResponses)
	return s
}

func (s *Server) Run(addr string) error {
	s.log.Info("proxy server starting",
		"addr", addr,
		"upstream", config.ProxyBaseURL(),
		"auths_dir", config.AuthsDir(),
	)
	return s.gin.Run(addr)
}

func (s *Server) handleResponses(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body failed"})
		return
	}
	model := proxy.ExtractModel(body)
	if model == "" {
		model = c.GetHeader("x-grok-model-override")
	}
	s.proxyRequest(c, func(rec auth.AccountRecord) (*http.Response, error) {
		return s.proxy.PostResponses(rec, body, model)
	}, true)
}

func (s *Server) proxyRequest(c *gin.Context, call func(auth.AccountRecord) (*http.Response, error), stream bool) {
	var resp *http.Response
	err := s.pool.WithRetry(func(rec auth.AccountRecord) (int, error) {
		r, err := call(rec)
		if err != nil {
			return 0, err
		}
		if retryStatus(r.StatusCode) {
			r.Body.Close()
			return r.StatusCode, nil
		}
		resp = r
		return r.StatusCode, nil
	})
	if err != nil {
		s.log.Error("proxy failed", "path", c.FullPath(), "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if stream {
		writeStream(c, resp, s.log)
		return
	}
	c.Status(resp.StatusCode)
	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	io.Copy(c.Writer, resp.Body)
}

func retryStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden || status >= http.StatusInternalServerError
}

func writeStream(c *gin.Context, resp *http.Response, log *slog.Logger) {
	c.Status(resp.StatusCode)
	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Header(k, v)
		}
	}

	flusher, _ := c.Writer.(http.Flusher)
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			c.Writer.Write(buf[:n])
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Warn("stream read error", "error", err)
			return
		}
	}
}
