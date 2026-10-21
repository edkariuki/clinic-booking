package domain

import (
	"time"
	_ "time/tzdata"
)

// fixed timezomne
var ClinicLocation = mustLoadLocation("Africa/Nairobi")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("domain: cannot load time zone " + name + ": " + err.Error())
	}
	return loc
}
