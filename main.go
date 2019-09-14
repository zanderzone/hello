package main

import (
    "encoding/json"
    "net/http"
		"strings"
		"log"
		"time"
)

type weatherProvider interface {
	temperature(city string) (float64, error) // in Kelvin, naturally
}

type multiWeatherProvider []weatherProvider

func hello(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("hello!"))
}

func query(city string) (weatherData, error) {
    resp, err := http.Get("http://api.openweathermap.org/data/2.5/weather?APPID=49eb535612df2ed169c36a3250095fef&q=" + city)
    if err != nil {
        return weatherData{}, err
    }

    defer resp.Body.Close()

    var d weatherData

    if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
        return weatherData{}, err
    }

    return d, nil
}

type weatherData struct {
    Name string `json:"name"`
    Main struct {
        Kelvin float64 `json:"temp"`
    } `json:"main"`
}

type openWeatherMap struct{}

func (w openWeatherMap) temperature(city string) (float64, error) {
    resp, err := http.Get("http://api.openweathermap.org/data/2.5/weather?APPID=49eb535612df2ed169c36a3250095fef&q=" + city)
    if err != nil {
        return 0, err
    }

    defer resp.Body.Close()

    var d struct {
        Main struct {
            Kelvin float64 `json:"temp"`
        } `json:"main"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
        return 0, err
    }

    log.Printf("openWeatherMap: %s: %.2f", city, d.Main.Kelvin)
    return d.Main.Kelvin, nil
}

func (w multiWeatherProvider) temperature(city string) (float64, error) {
	// Make a channel for temperatures, and a channel for errors.
	// Each provider will push a value into only one.
	temps := make(chan float64, len(w))
	errs := make(chan error, len(w))

	// For each provider, spawn a goroutine with an anonymous function.
	// That function will invoke the temperature method, and forward the response.
	for _, provider := range w {
			go func(p weatherProvider) {
					k, err := p.temperature(city)
					if err != nil {
							errs <- err
							return
					}
					temps <- k
			}(provider)
	}

	sum := 0.0

	// Collect a temperature or an error from each provider.
	for i := 0; i < len(w); i++ {
			select {
			case temp := <-temps:
					sum += temp
			case err := <-errs:
					return 0, err
			}
	}

	// Return the average, same as before.
	return sum / float64(len(w)), nil
}

func main() {
	mw := multiWeatherProvider{
			openWeatherMap{},
	}

	http.HandleFunc("/weather/", func(w http.ResponseWriter, r *http.Request) {
			begin := time.Now()
			city := strings.SplitN(r.URL.Path, "/", 3)[2]

			temp, err := mw.temperature(city)
			if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]interface{}{
					"city": city,
					"temp": temp,
					"took": time.Since(begin).String(),
			})
	})

	http.ListenAndServe(":8080", nil)
}