package lwm2m

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/diwise/integration-cip-havochvatten/internal/application/models"
	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestTemperature(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()

	ts := time.Now()
	id := uuid.New().String()

	pack, err := temperature(ctx, id, models.Temperature{
		NutsCode:   "SE234098",
		InternalID: id,
		Lat:        17.1,
		Lon:        62.2,
		Date:       ts,
		Temp:       22.4,
		Source:     "Hav och vatten",
	})

	is.NoErr(err)

	is.Equal(pack[0].BaseName, fmt.Sprintf("%s/3303/", id))
	is.Equal(pack[0].StringValue, TemperatureURN)
	is.Equal(pack[0].BaseTime, float64(ts.Unix()))

	pack.Normalize()

	is.Equal(pack[1].Name, fmt.Sprintf("%s/3303/5700", id))
	is.Equal(*pack[1].Value, float64(22.4))
}
