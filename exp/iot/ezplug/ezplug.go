package ezplug

import (
	"fmt"

	"github.com/heatxsink/x/exp/iot"
)

var (
	On     = "on"
	Off    = "off"
	Toggle = "toggle"
)

type EzPlug struct {
	iot *iot.IoT
}

func New(i *iot.IoT) *EzPlug {
	return &EzPlug{
		iot: i,
	}
}

func (ep *EzPlug) Topic(id string) string {
	return fmt.Sprintf("cmnd/EZPlug_%s/Power", id)
}

func (ep *EzPlug) On(topic string) error {
	return ep.iot.Publish(topic, 0, false, On)
}

func (ep *EzPlug) Off(topic string) error {
	return ep.iot.Publish(topic, 0, false, Off)
}

func (ep *EzPlug) Toggle(topic string) error {
	return ep.iot.Publish(topic, 0, false, Toggle)
}
