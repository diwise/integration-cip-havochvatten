package lwm2m

import (
	"fmt"
	"time"

	"github.com/diwise/integration-cip-havochvatten/internal/application/models"
	"github.com/diwise/senml"
)

type SenMLDecoratorFunc func(p *senML)

type senML struct {
	Pack senml.Pack
}

func NewSenMLPack(lwm2mObject models.Lwm2mObject, decorators ...SenMLDecoratorFunc) senml.Pack {
	s := &senML{}

	s.Pack = append(s.Pack, senml.Record{
		BaseName:    fmt.Sprintf("%s/%s/", lwm2mObject.ID(), lwm2mObject.ObjectID()),
		BaseTime:    float64(lwm2mObject.Timestamp().Unix()),
		Name:        "0",
		StringValue: lwm2mObject.ObjectURN(),
	})

	for _, d := range decorators {
		d(s)
	}

	return s.Pack
}

func Value(n string, v float64, unit string) SenMLDecoratorFunc {
	return Rec(n, &v, nil, "", nil, unit, nil)
}

func BoolValue(n string, vb bool) SenMLDecoratorFunc {
	return Rec(n, nil, nil, "", nil, "", &vb)
}

func Rec(n string, v, sum *float64, vs string, t *time.Time, u string, vb *bool) SenMLDecoratorFunc {
	var tm float64
	if t != nil {
		tm = float64(t.Unix())
	}

	return func(p *senML) {
		r := senml.Record{
			Name:        n,
			Unit:        u,
			Time:        tm,
			Value:       v,
			StringValue: vs,
			BoolValue:   vb,
			Sum:         sum,
		}
		p.Pack = append(p.Pack, r)
	}
}
