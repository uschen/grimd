package zapstackdriver

import (
	"errors"

	uuid "github.com/satori/go.uuid"

	"cloud.google.com/go/logging"
	"go.uber.org/zap/zapcore"
)

// Core is the core implements zapcore.Core
type Core struct {
	zapcore.LevelEnabler
	clogger *logging.Logger
	encoder *StructEncoder
}

// CoreOptionFunc -
type CoreOptionFunc func(*Core) error

// New -
func New(enab zapcore.LevelEnabler, cloudLogger *logging.Logger, options ...CoreOptionFunc) (*Core, error) {
	c := &Core{
		LevelEnabler: enab,
		clogger:      cloudLogger,
		encoder:      NewStructEncoder(),
	}
	if c.clogger == nil {
		return nil, errors.New("Cloud Logger is required")
	}

	// Run the options on it
	for _, option := range options {
		if err := option(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// With -
func (c *Core) With(fields []zapcore.Field) zapcore.Core {
	clone := c.clone()
	addFields(clone.encoder, fields)
	return clone
}

// Check -
func (c *Core) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

// Write -
func (c *Core) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	e2, err := c.encoder.encodeEntry(ent, fields)
	if err != nil {
		return err
	}
	e2.AddString("msg", ent.Message)
	if ent.Caller.Defined {
		e2.AddString("caller", ent.Caller.String())
	}
	if ent.Stack != "" {
		e2.AddString("stack", ent.Stack)
	}
	entry := logging.Entry{
		Timestamp: ent.Time,
		Severity:  zapLevelToSeverity(ent.Level),
		InsertID:  uuid.NewV1().String(),
		Payload:   e2.Struct,
	}

	c.clogger.Log(entry)
	return nil
}

// Sync -
func (c *Core) Sync() error {
	return nil
}

func (c *Core) clone() *Core {
	return &Core{
		LevelEnabler: c.LevelEnabler,
		encoder:      c.encoder.clone(),
		clogger:      c.clogger,
	}
}

func zapLevelToSeverity(level zapcore.Level) logging.Severity {
	switch level {
	case zapcore.DebugLevel:
		return logging.Debug
	case zapcore.InfoLevel:
		return logging.Info
	case zapcore.WarnLevel:
		return logging.Warning
	case zapcore.ErrorLevel:
		return logging.Error
	case zapcore.DPanicLevel:
		return logging.Critical
	case zapcore.PanicLevel:
		return logging.Alert
	case zapcore.FatalLevel:
		return logging.Emergency
	default:
		return logging.Info
	}
}
