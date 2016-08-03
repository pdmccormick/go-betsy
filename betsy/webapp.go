package betsy

import (
	"encoding/json"
	"fmt"
	"github.com/gocraft/web"
	"net/http"
	"strconv"
)

import _ "image/gif"
import _ "image/png"
import _ "image/jpeg"

const DEFAULT_MAX_POSTSCALER = 0.5

type WebApp struct {
	Net           *Network
	Display       *Display
	Settings      PWMSettings
	MaxPostscaler float32
	Router        *web.Router
}

type Context struct {
	app *WebApp
}

func (app *WebApp) setAppContext() interface{} {
	return func(c *Context, rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
		c.app = app
		next(rw, req)
	}
}

func (c *Context) SettingsList(rw web.ResponseWriter, req *web.Request) {
	type settingsResponse struct {
		Gamma         float64       `json:"gamma"`
		MaxBrightness float32       `json:"max_brightness"`
		Brightness    float32       `json:"brightness"`
		Transform     [3][3]float32 `json:"transform"`
	}

	s := settingsResponse{
		Gamma:         c.app.Settings.Gamma,
		MaxBrightness: c.app.MaxPostscaler,
		Brightness:    c.app.Settings.Postscaler,
		Transform:     [3][3]float32(c.app.Settings.Transform),
	}

	js, err := json.Marshal(s)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(js)
}

func (c *Context) SettingsUpdate(rw web.ResponseWriter, req *web.Request) {
	if p := req.FormValue("gamma"); p != "" {
		if val, err := strconv.ParseFloat(p, 64); err == nil {
			c.app.Settings.SetGamma(val)
		}
	}

	if p := req.FormValue("brightness"); p != "" {
		if val, err := strconv.ParseFloat(p, 64); err == nil {
			c.app.Settings.Postscaler = float32(val)
		}
	}

	if p := req.FormValue("max_brightness"); p != "" {
		if val, err := strconv.ParseFloat(p, 64); err == nil {
			c.app.MaxPostscaler = float32(val)
		}
	}

	// Accept `?transform=id` or `?transform=[[1,0,0],[0,1,0],[0,0,1]]`
	if p := req.FormValue("transform"); p != "" {
		if p == "id" {
			c.app.Settings.Transform = Identity3x3
		} else {
			var transform [3][3]float32
			err := json.Unmarshal([]byte(p), &transform)
			if err == nil {
				c.app.Settings.Transform = Matrix3x3(transform)
			}
		}
	}

	// Accept `?transform[2][2]=1` or `?transform[1]=[0,1,0]`
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			field := fmt.Sprintf("transform[%d][%d]", row, col)
			if p := req.FormValue(field); p != "" {
				if val, err := strconv.ParseFloat(p, 64); err == nil {
					c.app.Settings.Transform[row][col] = float32(val)
				}
			}
		}

		field := fmt.Sprintf("transform[%d]", row)
		if p := req.FormValue(field); p != "" {
			var vector [3]float32
			err := json.Unmarshal([]byte(p), &vector)
			if err == nil {
				c.app.Settings.Transform[row] = vector
			}
		}
	}

	c.SettingsList(rw, req)
}

func (c *Context) SettingsReset(rw web.ResponseWriter, req *web.Request) {
	c.app.Settings = *DefaultPWMSettings
	c.app.MaxPostscaler = DEFAULT_MAX_POSTSCALER

	c.SettingsList(rw, req)
}

func MakeWebApp(ifname string) (*WebApp, error) {
	network, err := NetworkByInterfaceName(ifname)
	if err != nil {
		return nil, err
	}

	settings := *DefaultPWMSettings

	app := &WebApp{
		Net: network,
		Display: &Display{
			Net: network,
		},
		Settings:      settings,
		MaxPostscaler: DEFAULT_MAX_POSTSCALER,
	}

	app.Router = web.New(Context{}).
		Middleware(app.setAppContext()).
		Middleware(web.LoggerMiddleware).
		Middleware(web.ShowErrorsMiddleware).
		Get("/settings", (*Context).SettingsList).
		Post("/settings", (*Context).SettingsUpdate).
		Post("/settings/reset", (*Context).SettingsReset)

	return app, nil
}
