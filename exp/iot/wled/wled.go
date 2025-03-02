package wled

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/heatxsink/x/exp/iot"
)

var (
	On       = "ON"
	Off      = "OFF"
	Toggle   = "T"
	TopicAll = "wled/all"
)

type LightInfo struct {
	State struct {
		On         bool `json:"on,omitempty"`
		Bri        int  `json:"bri,omitempty"`
		Transition int  `json:"transition,omitempty"`
		Ps         int  `json:"ps,omitempty"`
		Pl         int  `json:"pl,omitempty"`
		Ledmap     int  `json:"ledmap,omitempty"`
		Nl         struct {
			On   bool `json:"on,omitempty"`
			Dur  int  `json:"dur,omitempty"`
			Mode int  `json:"mode,omitempty"`
			Tbri int  `json:"tbri,omitempty"`
			Rem  int  `json:"rem,omitempty"`
		} `json:"nl,omitempty"`
		Udpn struct {
			Send bool `json:"send,omitempty"`
			Recv bool `json:"recv,omitempty"`
			Sgrp int  `json:"sgrp,omitempty"`
			Rgrp int  `json:"rgrp,omitempty"`
		} `json:"udpn,omitempty"`
		Lor     int `json:"lor,omitempty"`
		Mainseg int `json:"mainseg,omitempty"`
		Seg     []struct {
			ID    int     `json:"id,omitempty"`
			Start int     `json:"start,omitempty"`
			Stop  int     `json:"stop,omitempty"`
			Len   int     `json:"len,omitempty"`
			Grp   int     `json:"grp,omitempty"`
			Spc   int     `json:"spc,omitempty"`
			Of    int     `json:"of,omitempty"`
			On    bool    `json:"on,omitempty"`
			Frz   bool    `json:"frz,omitempty"`
			Bri   int     `json:"bri,omitempty"`
			Cct   int     `json:"cct,omitempty"`
			Set   int     `json:"set,omitempty"`
			Col   [][]int `json:"col,omitempty"`
			Fx    int     `json:"fx,omitempty"`
			Sx    int     `json:"sx,omitempty"`
			Ix    int     `json:"ix,omitempty"`
			Pal   int     `json:"pal,omitempty"`
			C1    int     `json:"c1,omitempty"`
			C2    int     `json:"c2,omitempty"`
			C3    int     `json:"c3,omitempty"`
			Sel   bool    `json:"sel,omitempty"`
			Rev   bool    `json:"rev,omitempty"`
			Mi    bool    `json:"mi,omitempty"`
			O1    bool    `json:"o1,omitempty"`
			O2    bool    `json:"o2,omitempty"`
			O3    bool    `json:"o3,omitempty"`
			Si    int     `json:"si,omitempty"`
			M12   int     `json:"m12,omitempty"`
		} `json:"seg,omitempty"`
	} `json:"state,omitempty"`
	Info struct {
		Ver     string `json:"ver,omitempty"`
		Vid     int    `json:"vid,omitempty"`
		Cn      string `json:"cn,omitempty"`
		Release string `json:"release,omitempty"`
		Leds    struct {
			Count  int   `json:"count,omitempty"`
			Pwr    int   `json:"pwr,omitempty"`
			Fps    int   `json:"fps,omitempty"`
			Maxpwr int   `json:"maxpwr,omitempty"`
			Maxseg int   `json:"maxseg,omitempty"`
			Bootps int   `json:"bootps,omitempty"`
			Seglc  []int `json:"seglc,omitempty"`
			Lc     int   `json:"lc,omitempty"`
			Rgbw   bool  `json:"rgbw,omitempty"`
			Wv     int   `json:"wv,omitempty"`
			Cct    int   `json:"cct,omitempty"`
		} `json:"leds,omitempty"`
		Str          bool   `json:"str,omitempty"`
		Name         string `json:"name,omitempty"`
		Udpport      int    `json:"udpport,omitempty"`
		Simplifiedui bool   `json:"simplifiedui,omitempty"`
		Live         bool   `json:"live,omitempty"`
		Liveseg      int    `json:"liveseg,omitempty"`
		Lm           string `json:"lm,omitempty"`
		Lip          string `json:"lip,omitempty"`
		Ws           int    `json:"ws,omitempty"`
		Fxcount      int    `json:"fxcount,omitempty"`
		Palcount     int    `json:"palcount,omitempty"`
		Cpalcount    int    `json:"cpalcount,omitempty"`
		Maps         []struct {
			ID int `json:"id,omitempty"`
		} `json:"maps,omitempty"`
		Wifi struct {
			Bssid   string `json:"bssid,omitempty"`
			Rssi    int    `json:"rssi,omitempty"`
			Signal  int    `json:"signal,omitempty"`
			Channel int    `json:"channel,omitempty"`
			Ap      bool   `json:"ap,omitempty"`
		} `json:"wifi,omitempty"`
		Fs struct {
			U   int `json:"u,omitempty"`
			T   int `json:"t,omitempty"`
			Pmt int `json:"pmt,omitempty"`
		} `json:"fs,omitempty"`
		Ndc      int    `json:"ndc,omitempty"`
		Arch     string `json:"arch,omitempty"`
		Core     string `json:"core,omitempty"`
		Clock    int    `json:"clock,omitempty"`
		Flash    int    `json:"flash,omitempty"`
		Lwip     int    `json:"lwip,omitempty"`
		Freeheap int    `json:"freeheap,omitempty"`
		Uptime   int    `json:"uptime,omitempty"`
		Time     string `json:"time,omitempty"`
		Opt      int    `json:"opt,omitempty"`
		Brand    string `json:"brand,omitempty"`
		Product  string `json:"product,omitempty"`
		Mac      string `json:"mac,omitempty"`
		IP       string `json:"ip,omitempty"`
	} `json:"info,omitempty"`
	Effects  []string `json:"effects,omitempty"`
	Palettes []string `json:"palettes,omitempty"`
}

type WLed struct {
	iot *iot.IoT
}

func New(i *iot.IoT) *WLed {
	return &WLed{
		iot: i,
	}
}

func (w *WLed) On(topic string) error {
	return w.iot.Publish(topic, 0, false, On)
}

func (w *WLed) Off(topic string) error {
	return w.iot.Publish(topic, 0, false, Off)
}

func (w *WLed) Toggle(topic string) error {
	return w.iot.Publish(topic, 0, false, Toggle)
}

func (w *WLed) Brightness(topic string, value int64) error {
	return w.iot.Publish(topic, 0, false, strconv.FormatInt(value, 10))
}

func (w *WLed) Color(topic string, value string) error {
	return w.iot.Publish(topic+"/col", 0, false, value)
}

func (w *WLed) API(topic string, value string) error {
	return w.iot.Publish(topic+"/api", 0, false, value)
}

func (w *WLed) PulseN(topic string, pulse int) error {
	if err := w.On(topic); err != nil {
		return err
	}
	for i := 1; i <= (pulse * 2); i++ {
		if i%2 == 0 {
			if err := w.On(topic); err != nil {
				return err
			}
		} else {
			if err := w.Off(topic); err != nil {
				return err
			}
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func Info(ctx context.Context, hostname string) (*LightInfo, error) {
	url := fmt.Sprintf("http://%s/json/si", hostname)
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{}
	client.Timeout = time.Second * 20
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP status code: %d", response.StatusCode)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var li *LightInfo
	err = json.Unmarshal(body, &li)
	if err != nil {
		return nil, err
	}
	return li, nil
}
