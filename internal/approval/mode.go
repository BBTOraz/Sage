package approval

type Mode string

const (
	Guard Mode = "guard"
	Full  Mode = "full"
)

func (m Mode) String() string {
	return string(m)
}

func (m Mode) Valid() bool {
	return m == Guard || m == Full
}
func (m Mode) GetDefault() Mode {
	return Guard
}
