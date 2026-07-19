package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/D3wier/race-the-web/pkg/config"
	"github.com/D3wier/race-the-web/pkg/racer"
)

var banner = `
╔══════════════════════════════════════════════════════╗
║  race-the-web v0.1.0 — HTTP Race Condition Tester  ║
╚══════════════════════════════════════════════════════╝
`

type headerFlags []string

func (h *headerFlags) String() string { return strings.Join(*h, ", ") }
func (h *headerFlags) Set(value string) error {
	*h = append(*h, value)
	return nil
}

func main() {
	var headers headerFlags

	url := flag.String("u", "", "Target URL")
	method := flag.String("m", "GET", "HTTP method")
	count := flag.Int("n", 20, "Number of parallel requests")
	body := flag.String("d", "", "Request body")
	cookie := flag.String("cookie", "", "Cookie header")
	configFile := flag.String("c", "", "Config file (YAML)")
	rounds := flag.Int("rounds", 1, "Number of rounds")
	delay := flag.Duration("delay", time.Second, "Delay between rounds")
	strategy := flag.String("strategy", "barrier", "Sync strategy: barrier|lastbyte|pipeline")
	useHTTP2 := flag.Bool("http2", false, "Force HTTP/2")
	timeout := flag.Duration("timeout", 10*time.Second, "Request timeout")
	verbose := flag.Bool("v", false, "Verbose output")
	output := flag.String("o", "", "Output file (JSON)")
	flag.Var(&headers, "H", "Header (repeatable)")
	flag.Parse()

	fmt.Print(banner)

	var cfg *config.Config

	if *configFile != "" {
		var err error
		cfg, err = config.LoadFile(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
	} else if *url != "" {
		headerMap := make(map[string]string)
		for _, h := range headers {
			parts := strings.SplitN(h, ":", 2)
			if len(parts) == 2 {
				headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		if *cookie != "" {
			headerMap["Cookie"] = *cookie
		}

		cfg = &config.Config{
			Target:   *url,
			Strategy: *strategy,
			Rounds:   *rounds,
			Delay:    *delay,
			HTTP2:    *useHTTP2,
			Timeout:  *timeout,
			Requests: []config.RequestConfig{
				{
					Name:    "race",
					Method:  *method,
					URL:     *url,
					Headers: headerMap,
					Body:    *body,
					Count:   *count,
				},
			},
		}
	} else {
		fmt.Fprintln(os.Stderr, "Error: provide -u <url> or -c <config.yaml>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	r := racer.New(cfg, *verbose)

	fmt.Printf("Target: %s %s\n", cfg.Requests[0].Method, cfg.Target)
	fmt.Printf("Strategy: %s | Requests: %d | Rounds: %d\n\n",
		cfg.Strategy, cfg.Requests[0].Count, cfg.Rounds)

	allResults := r.Run()

	if *output != "" {
		racer.SaveResults(allResults, *output)
		fmt.Printf("\nResults saved to: %s\n", *output)
	}
}
