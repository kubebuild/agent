package app

import "github.com/Sirupsen/logrus"

// Configurer interface allows individual app config structs to inherit Fields
// from Config and still be used by the agent .
type Configurer interface {
	GetName() string
	GetVersion() string
	GetToken() string
	GetLogLevel() logrus.Level
	GetGraphqlURL() string
}

// Config contains the base configuration fields required for the agent app.
type Config struct {
	Name       string
	Version    string
	Token      string
	LogLevel   string
	GraphqlURL string
}

// GetName app name.
func (c *Config) GetName() string {
	return c.Name
}

// GetGraphqlURL app name.
func (c *Config) GetGraphqlURL() string {
	return c.GraphqlURL
}

// GetVersion app version.
func (c *Config) GetVersion() string {
	return c.Version
}

// GetToken app token.
func (c *Config) GetToken() string {
	return c.Token
}

// GetLogLevel parses and returns the log level, defaulting to Info.
func (c *Config) GetLogLevel() logrus.Level {
	level, _ := logrus.ParseLevel(c.LogLevel)
	if level == 0 {
		level = logrus.InfoLevel
	}
	return level
}
