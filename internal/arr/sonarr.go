package arr

import (
	"fmt"
	"path"

	"github.com/fsnotify/fsnotify"
	"github.com/samjwillis97/sams-blackhole/internal/torrents"
)

func SonarrHandler(e fsnotify.Event, root string) {
	filepath := path.Join(root, e.Name)
	switch e.Op {
	case fsnotify.Create:
		handleNewSonarrFile(filepath)
	}
}

func handleNewSonarrFile(filepath string) {
	fmt.Printf("Sonarr handle created %s\n", filepath)
	toProcess := torrents.New(filepath)
	fmt.Println(toProcess)
  // TODO: Move to processing directory
  // TODO: ???
}
