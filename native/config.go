package native

// native/config.go — 本文件为本项目自己写的代码，不是微软的代码。
// This file is written for this project; no Microsoft code is used here.
//
// Settings are persisted by config.dll as a TMMF document, written to
// <dir>/resources/cmd/settings.tmmf:
//
//	#MDRenderer Settings
//	# <timestamp>
//	language=en
//	log_enabled=0
//
// config_set_setting only changes memory. config_save_settings is what writes
// the file, so Set alone is not durable.

// Config wraps config.dll. A nil *Config is valid and does nothing, which lets
// callers keep working when the DLL is missing.
type Config struct {
	d *DLL
}

var configExports = []string{
	"graal_create_isolate", "graal_detach_all_threads_and_tear_down_isolate",
	"config_init", "config_load_settings", "config_save_settings",
	"config_get_setting", "config_set_setting", "config_has_setting",
	"config_remove_setting", "config_get_thread_mode", "config_set_thread_mode",
}

// NewConfig loads config.dll from dir, initialises it and loads the settings
// that are already on disk. It returns nil when the DLL is unavailable.
func NewConfig(dir string) *Config {
	d := Load(dir, "config.dll", configExports)
	if !d.Valid() {
		return nil
	}
	d.Invoke("config_init", Str(dir))
	d.Invoke("config_load_settings")
	return &Config{d: d}
}

// Valid reports whether the settings store is usable.
func (c *Config) Valid() bool { return c != nil && c.d.Valid() }

// Get returns the stored value, or "" when the key is absent or unavailable.
func (c *Config) Get(key string) string {
	if !c.Valid() {
		return ""
	}
	buf := make([]byte, 4096)
	out := Buf(buf)
	c.d.Invoke("config_get_setting", Str(key), out, Num(uintptr(len(buf))))
	return out.Out()
}

// Set writes a value into memory. Call Save to persist it.
func (c *Config) Set(key, value string) {
	if !c.Valid() {
		return
	}
	c.d.Invoke("config_set_setting", Str(key), Str(value))
}

// Save flushes the in-memory settings to resources/cmd/settings.tmmf.
func (c *Config) Save() {
	if !c.Valid() {
		return
	}
	c.d.Invoke("config_save_settings")
}

// Close tears the isolate down.
func (c *Config) Close() {
	if c == nil {
		return
	}
	c.d.Close()
}
