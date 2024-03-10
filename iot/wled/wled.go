package wled

import (
	"strconv"
	"time"

	"github.com/heatxsink/x/iot"
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
		Nl         struct {
			On   bool `json:"on,omitempty"`
			Dur  int  `json:"dur,omitempty"`
			Fade bool `json:"fade,omitempty"`
			Tbri int  `json:"tbri,omitempty"`
		} `json:"nl,omitempty"`
		Udpn struct {
			Send bool `json:"send,omitempty"`
			Recv bool `json:"recv,omitempty"`
		} `json:"udpn,omitempty"`
		Seg []struct {
			Start int     `json:"start,omitempty"`
			Stop  int     `json:"stop,omitempty"`
			Len   int     `json:"len,omitempty"`
			Col   [][]int `json:"col,omitempty"`
			Fx    int     `json:"fx,omitempty"`
			Sx    int     `json:"sx,omitempty"`
			Ix    int     `json:"ix,omitempty"`
			Pal   int     `json:"pal,omitempty"`
			Sel   bool    `json:"sel,omitempty"`
			Rev   bool    `json:"rev,omitempty"`
			Cln   int     `json:"cln,omitempty"`
		} `json:"seg,omitempty"`
	} `json:"state,omitempty"`
	Info struct {
		Ver  string `json:"ver,omitempty"`
		Vid  int    `json:"vid,omitempty"`
		Leds struct {
			Count  int   `json:"count,omitempty"`
			Rgbw   bool  `json:"rgbw,omitempty"`
			Pin    []int `json:"pin,omitempty"`
			Pwr    int   `json:"pwr,omitempty"`
			Maxpwr int   `json:"maxpwr,omitempty"`
			Maxseg int   `json:"maxseg,omitempty"`
		} `json:"leds,omitempty"`
		Name     string `json:"name,omitempty"`
		Udpport  int    `json:"udpport,omitempty"`
		Live     bool   `json:"live,omitempty"`
		Fxcount  int    `json:"fxcount,omitempty"`
		Palcount int    `json:"palcount,omitempty"`
		Arch     string `json:"arch,omitempty"`
		Core     string `json:"core,omitempty"`
		Freeheap int    `json:"freeheap,omitempty"`
		Uptime   int    `json:"uptime,omitempty"`
		Opt      int    `json:"opt,omitempty"`
		Brand    string `json:"brand,omitempty"`
		Product  string `json:"product,omitempty"`
		Btype    string `json:"btype,omitempty"`
		Mac      string `json:"mac,omitempty"`
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
