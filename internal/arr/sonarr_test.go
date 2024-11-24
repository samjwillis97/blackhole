package arr_test

import (
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/arr"
)

func TestSonarrCreationHandler(t *testing.T) {
	event := fsnotify.Event{
		Name: "testFile",
		Op:   fsnotify.Create,
	}

	arr.SonarrHandler(event, "/tmp")
}
