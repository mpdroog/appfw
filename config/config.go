package config

type Config struct {
	/* Path/to/state */
	State string `default:"./state.tsv"`
	/* Amount of entries to keep track of */
	StateSize int `default:1024`

	/** Host:port addr */
	Listen string `default:"127.0.0.1:1337"`
	/* Request limit per minute */
	Ratelimit int `default:"50"`
	/* Restrict powertools with apikey */
	APIKey string
}

var (
	Verbose bool
	C       Config
)
