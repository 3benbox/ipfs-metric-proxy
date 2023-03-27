package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Define the metrics we wish to expose
var (
	requestURLPath = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "request_url_path",
			Help: "Proxied request paths."},
		[]string{"path"},
	)
	responseStatus = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "response_status",
			Help: "Proxied request's response status."},
		[]string{"code", "path"},
	)
	responseErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "response_errors",
			Help: "Proxied request's response errors."},
	)
)

type SimpleProxy struct {
	Proxy *httputil.ReverseProxy
}

func NewProxy(rawUrl string) (*SimpleProxy, error) {
	url, err := url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}
	s := &SimpleProxy{httputil.NewSingleHostReverseProxy(url)}

	// Modify requests
	originalDirector := s.Proxy.Director
	s.Proxy.Director = func(r *http.Request) {
		originalDirector(r)
	}

	// Modify response
	s.Proxy.ModifyResponse = func(r *http.Response) error {
		log.Printf("Upstream returned status code %v", r.StatusCode)
		updateResponseStatusMetric(r.Status, r.Request.RequestURI)
		return nil
	}

	return s, nil
}

func (s *SimpleProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Proxy receives request %v", r.URL.Path)
	updateRequestURLPathMetric(r.URL.Path)
	log.Printf("Proxy forwards request to origin.")
	s.Proxy.ServeHTTP(w, r)
	log.Printf("Origin server completes request.")
}

func updateResponseStatusMetric(code string, path string) {
	u, err := url.Parse(path)
	if err != nil {
		responseErrors.Inc()
		return
	}
	responseStatus.WithLabelValues(code, u.Path).Inc()
}

func updateRequestURLPathMetric(path string) {
	monitored_urls := make(map[string]bool)
	monitored_urls["/api/v0/pin/add"] = true
	monitored_urls["/api/v0/pin/rm"] = true
	monitored_urls["/api/v0/pin/ls"] = true
	monitored_urls["/api/v0/id"] = true
	monitored_urls["/api/v0/pubsub/ls"] = true
	monitored_urls["/api/v0/pubsub/pub"] = true
	monitored_urls["/api/v0/pubsub/sub"] = true
	monitored_urls["/api/v0/dag/get"] = true
	monitored_urls["/api/v0/dag/put"] = true
	monitored_urls["/api/v0/dag/resolve"] = true
	monitored_urls["/api/v0/block/put"] = true
	monitored_urls["/api/v0/block/get"] = true
	monitored_urls["/api/v0/block/stat"] = true
	monitored_urls["/api/v0/swarm/peers"] = true
	monitored_urls["/api/v0/swarm/connect"] = true

	if monitored_urls[path] {
		requestURLPath.WithLabelValues(path).Inc()
	}
	requestURLPath.WithLabelValues("unmonitored").Inc()
}

func main() {
	listenAddress := os.Getenv("LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = ":9100"
	}

	ipfsApiUrl := os.Getenv("IPFS_API_URL")
	if ipfsApiUrl == "" {
		log.Fatal("IPFS_API_URL environmental variable is required")
		os.Exit(1)
	}

	proxy, err := NewProxy(ipfsApiUrl)
	if err != nil {
		panic(err)
	}

	http.Handle("/", proxy)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(listenAddress, nil))
}

func init() {
	//Register metrics with prometheus
	prometheus.MustRegister(requestURLPath)
	prometheus.MustRegister(responseStatus)
	prometheus.MustRegister(responseErrors)
}
