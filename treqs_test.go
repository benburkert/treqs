package treqs

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
)

func ExampleTracer() {
	piHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rounds, err := strconv.Atoi(r.FormValue("rounds"))
		if err != nil {
			rounds = 50000
		}

		ch := make(chan float64)
		for k := float64(1); k <= float64(rounds); k++ {
			go func(k float64) { ch <- (-4) * math.Pow(-1, k) / (2 * k * (2*k + 1) * (2*k + 2)) }(k)
		}

		pi := 3.0
		for k := 1; k <= rounds; k++ {
			pi += <-ch
		}

		w.Write([]byte(fmt.Sprintf("π to the 10th digit is ~%.10f\n", pi)))
	})

	srv := httptest.NewServer(&Tracer{
		Key:     "treqs",
		Handler: piHandler,
	})

	req, err := http.NewRequest("GET", srv.URL, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("X-Treqs-Key", "treqs")
	req.Header.Set("X-Treqs-Action", "trace")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, res.Body)

	traceID := res.Header.Get("X-Treqs-Id")
	if traceID == "" {
		panic("missing X-Treqs-Id header")
	}

	if req, err = http.NewRequest("GET", srv.URL, nil); err != nil {
		panic(err)
	}
	req.Header.Set("X-Treqs-Key", "treqs")
	req.Header.Set("X-Treqs-Action", "read")
	req.Header.Set("X-Treqs-ID", traceID)

	if res, err = http.DefaultClient.Do(req); err != nil {
		panic(err)
	}
	if res.StatusCode != http.StatusOK {
		panic("unexpected response: " + res.Status)
	}

	file, err := os.Create("pi.trace")
	if err != nil {
		panic(err)
	}
	if _, err := io.Copy(file, res.Body); err != nil {
		panic(err)
	}

	// Output:
	// π to the 10th digit is ~3.1415926536

	// run "go tool trace pi.trace" to view the trace in Chrome
}
