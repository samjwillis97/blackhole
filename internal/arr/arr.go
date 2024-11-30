package arr

type ArrService int

const (
	Sonarr ArrService = iota
)

func (s ArrService) String() string {
	switch s {
	case Sonarr:
		return "Sonarr"
	}

	return "Unknown"
}
