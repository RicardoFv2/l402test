package config

import (
	"testing"
)

func TestValidate(t *testing.T) {
	t.Parallel()
	valid := Config{Version: CurrentVersion, Endpoints: []Endpoint{{
		Name: "weather", Method: "GET", Path: "/v1/weather", PriceSats: 21,
		Response: Response{Status: 200, ContentType: "application/json", Body: "{}"},
	}}}
	for _, testCase := range []struct {
		name string
		cfg  Config
		want bool
	}{
		{name: "valid", cfg: valid},
		{name: "wrong version", cfg: Config{Version: 2, Endpoints: valid.Endpoints}, want: true},
		{name: "missing endpoints", cfg: Config{Version: 1}, want: true},
		{name: "relative path", cfg: Config{Version: 1, Endpoints: []Endpoint{{Name: "x", Method: "GET", Path: "weather", PriceSats: 1, Response: valid.Endpoints[0].Response}}}, want: true},
		{name: "zero price", cfg: Config{Version: 1, Endpoints: []Endpoint{{Name: "x", Method: "GET", Path: "/weather", Response: valid.Endpoints[0].Response}}}, want: true},
		{name: "duplicate route", cfg: Config{Version: 1, Endpoints: []Endpoint{valid.Endpoints[0], valid.Endpoints[0]}}, want: true},
		{name: "duplicate name", cfg: Config{Version: 1, Endpoints: []Endpoint{valid.Endpoints[0], {Name: "weather", Method: "POST", Path: "/summary", PriceSats: 10, Response: valid.Endpoints[0].Response}}}, want: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			err := Validate(testCase.cfg)
			if (err != nil) != testCase.want {
				t.Fatalf("Validate() error = %v, want error %v", err, testCase.want)
			}
		})
	}
}
