package utils

import "github.com/Sirupsen/logrus"

//LogError logs the error
func LogError(message string, err error, log *logrus.Logger) {
	if err != nil {
		log.WithError(err).Error(message)
	}
}
