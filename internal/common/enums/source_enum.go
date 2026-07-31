package enums

type SourceType string

const (
	Local      SourceType = "local"
	Bangumi    SourceType = "bangumi"
	VNDB       SourceType = "vndb"
	Ymgal      SourceType = "ymgal"
	Steam      SourceType = "steam"
	DLsite     SourceType = "dlsite"
	TouchGal   SourceType = "touchgal"
	Hikarinagi SourceType = "hikarinagi"

	ErogameScape SourceType = "erogamescape"
)

var AllSourceTypes = []struct {
	Value  SourceType
	TSName string
}{
	{Local, "LOCAL"},
	{Bangumi, "BANGUMI"},
	{VNDB, "VNDB"},
	{Ymgal, "YMGAL"},
	{Steam, "STEAM"},
	{DLsite, "DLSITE"},
	{TouchGal, "TOUCHGAL"},
	{Hikarinagi, "HIKARINAGI"},
	{ErogameScape, "EROGAMESCAPE"},
}
