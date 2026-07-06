// Package distribution implements a logger which compatible to logrus/std log/prometheus.
package distribution

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"go.uber.org/zap"

	logruslogger "github.com/wangweihong/gotoolbox/pkg/log/logrus"
)

// Logger is a logger which compatible to logrus/std log/prometheus.
// it implements Print() Println() Printf() Dbug() Debugln() Debugf() Info() Infoln() Infof() Warn() Warnln() Warnf()
// Error() Errorln() Errorf() Fatal() Fataln() Fatalf() Panic() Panicln() Panicf() With() WithField() WithFields().
type Logger struct {
	logger       *zap.Logger
	logrusLogger *logrus.Logger
}

// NewLogger create the field logger object by giving zap logger.
func NewLogger(logger *zap.Logger) *Logger {
	return &Logger{
		logger:       logger,
		logrusLogger: logruslogger.NewLogger(logger),
	}
}

// Print logs a message at level Print on the compatibleLogger.
func (l *Logger) Print(args ...any) {
	l.logger.Info(fmt.Sprint(args...))
}

// Println logs a message at level Print on the compatibleLogger.
func (l *Logger) Println(args ...any) {
	l.logger.Info(fmt.Sprint(args...))
}

// Printf logs a message at level Print on the compatibleLogger.
func (l *Logger) Printf(format string, args ...any) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

// Trace logs a message at level Trace on the compatibleLogger.
func (l *Logger) Trace(args ...any) {
	l.logger.Debug(fmt.Sprint(args...))
}

// Traceln logs a message at level Trace on the compatibleLogger.
func (l *Logger) Traceln(args ...any) {
	l.logger.Debug(fmt.Sprint(args...))
}

// Tracef logs a message at level Trace on the compatibleLogger.
func (l *Logger) Tracef(format string, args ...any) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}

// Debug logs a message at level Debug on the compatibleLogger.
func (l *Logger) Debug(args ...any) {
	l.logger.Debug(fmt.Sprint(args...))
}

// Debugln logs a message at level Debug on the compatibleLogger.
func (l *Logger) Debugln(args ...any) {
	l.logger.Debug(fmt.Sprint(args...))
}

// Debugf logs a message at level Debug on the compatibleLogger.
func (l *Logger) Debugf(format string, args ...any) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}

// Info logs a message at level Info on the compatibleLogger.
func (l *Logger) Info(args ...any) {
	l.logger.Info(fmt.Sprint(args...))
}

// Infoln logs a message at level Info on the compatibleLogger.
func (l *Logger) Infoln(args ...any) {
	l.logger.Info(fmt.Sprint(args...))
}

// Infof logs a message at level Info on the compatibleLogger.
func (l *Logger) Infof(format string, args ...any) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

// Warn logs a message at level Warn on the compatibleLogger.
func (l *Logger) Warn(args ...any) {
	l.logger.Warn(fmt.Sprint(args...))
}

// Warnln logs a message at level Warn on the compatibleLogger.
func (l *Logger) Warnln(args ...any) {
	l.logger.Warn(fmt.Sprint(args...))
}

// Warnf logs a message at level Warn on the compatibleLogger.
func (l *Logger) Warnf(format string, args ...any) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}

// Warning logs a message at level Warn on the compatibleLogger.
func (l *Logger) Warning(args ...any) {
	l.logger.Warn(fmt.Sprint(args...))
}

// Warningln logs a message at level Warning on the compatibleLogger.
func (l *Logger) Warningln(args ...any) {
	l.logger.Warn(fmt.Sprint(args...))
}

// Warningf logs a message at level Warning on the compatibleLogger.
func (l *Logger) Warningf(format string, args ...any) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}

// Error logs a message at level Error on the compatibleLogger.
func (l *Logger) Error(args ...any) {
	l.logger.Error(fmt.Sprint(args...))
}

// Errorln logs a message at level Error on the compatibleLogger.
func (l *Logger) Errorln(args ...any) {
	l.logger.Error(fmt.Sprint(args...))
}

// Errorf logs a message at level Error on the compatibleLogger.
func (l *Logger) Errorf(format string, args ...any) {
	l.logger.Error(fmt.Sprintf(format, args...))
}

// Fatal logs a message at level Fatal on the compatibleLogger.
func (l *Logger) Fatal(args ...any) {
	l.logger.Fatal(fmt.Sprint(args...))
}

// Fatalln logs a message at level Fatal on the compatibleLogger.
func (l *Logger) Fatalln(args ...any) {
	l.logger.Fatal(fmt.Sprint(args...))
}

// Fatalf logs a message at level Fatal on the compatibleLogger.
func (l *Logger) Fatalf(format string, args ...any) {
	l.logger.Fatal(fmt.Sprintf(format, args...))
}

// Panic logs a message at level Painc on the compatibleLogger.
func (l *Logger) Panic(args ...any) {
	l.logger.Panic(fmt.Sprint(args...))
}

// Panicln logs a message at level Painc on the compatibleLogger.
func (l *Logger) Panicln(args ...any) {
	l.logger.Panic(fmt.Sprint(args...))
}

// Panicf logs a message at level Painc on the compatibleLogger.
func (l *Logger) Panicf(format string, args ...any) {
	l.logger.Panic(fmt.Sprintf(format, args...))
}

// WithError return a logger with an error field.
func (l *Logger) WithError(err error) *logrus.Entry {
	return logrus.NewEntry(l.logrusLogger).WithError(err)
}
