package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const configurationVersion = 2

// SavedTest is the test configuration remembered after an intentional Start.
// DurationSeconds and Count are both retained when switching modes, although
// only the active mode's length controls a test.
type SavedTest struct {
	Mode            string        `json:"mode"`
	Pack            string        `json:"pack"`
	DurationSeconds int           `json:"durationSeconds"`
	Count           int           `json:"count"`
	Modifiers       TestModifiers `json:"modifiers"`
	Difficulty      string        `json:"difficulty"`
}

// Appearance contains the presentation preferences in the version-2 file.
type Appearance struct {
	Theme           string `json:"theme"`
	Focus           bool   `json:"focus"`
	ShowErrors      bool   `json:"showErrors"`
	ReducedMotion   bool   `json:"reducedMotion"`
	KeySound        string `json:"keySound"`
	ErrorSound      string `json:"errorSound"`
	CompletionSound string `json:"completionSound"`
	PBSound         string `json:"pbSound"`
}

// Configuration is the complete version-2 settings document.
type Configuration struct {
	Version         int             `json:"version"`
	Settings        runtimeSettings `json:"settings"`
	Test            SavedTest       `json:"test"`
	Appearance      Appearance      `json:"appearance"`
	DiscoveredHints []string        `json:"discoveredHints"`
}

type testModifiersWire struct {
	Punctuation    *bool `json:"punctuation"`
	Numbers        *bool `json:"numbers"`
	Capitalization *bool `json:"capitalization"`
}

type savedTestWire struct {
	Mode            *string            `json:"mode"`
	Pack            *string            `json:"pack"`
	DurationSeconds *int               `json:"durationSeconds"`
	Count           *int               `json:"count"`
	Modifiers       *testModifiersWire `json:"modifiers"`
	Difficulty      *string            `json:"difficulty"`
}

type appearanceWire struct {
	Theme           *string `json:"theme"`
	Focus           *bool   `json:"focus"`
	ShowErrors      *bool   `json:"showErrors"`
	ReducedMotion   *bool   `json:"reducedMotion"`
	KeySound        *string `json:"keySound"`
	ErrorSound      *string `json:"errorSound"`
	CompletionSound *string `json:"completionSound"`
	PBSound         *string `json:"pbSound"`
}

type configurationWire struct {
	Version         *int                 `json:"version"`
	Settings        *runtimeSettingsWire `json:"settings"`
	Test            *savedTestWire       `json:"test"`
	Appearance      *appearanceWire      `json:"appearance"`
	DiscoveredHints *[]string            `json:"discoveredHints"`
}

type configurationJSON struct {
	Version         int             `json:"version"`
	Settings        runtimeSettings `json:"settings"`
	Test            savedTestJSON   `json:"test"`
	Appearance      Appearance      `json:"appearance"`
	DiscoveredHints []string        `json:"discoveredHints"`
}

type savedTestJSON struct {
	Mode            string            `json:"mode"`
	Pack            string            `json:"pack"`
	DurationSeconds int               `json:"durationSeconds"`
	Count           int               `json:"count"`
	Modifiers       testModifiersJSON `json:"modifiers"`
	Difficulty      string            `json:"difficulty"`
}

type testModifiersJSON struct {
	Punctuation    bool `json:"punctuation"`
	Numbers        bool `json:"numbers"`
	Capitalization bool `json:"capitalization"`
}

type legacyConfigurationWire struct {
	Version  *int                 `json:"version"`
	Settings *runtimeSettingsWire `json:"settings"`
}

func DefaultConfiguration() Configuration {
	return Configuration{
		Version:  configurationVersion,
		Settings: defaultRuntimeSettings(),
		Test: SavedTest{
			Mode:            "timed",
			Pack:            "1000en",
			DurationSeconds: 30,
			Count:           50,
			Difficulty:      "normal",
		},
		Appearance:      Appearance{Theme: "tt-dark"},
		DiscoveredHints: []string{},
	}
}

// MarshalJSON keeps TestModifiers on the exact lower-camel-case version-2
// schema without changing the legacy command-line type that owns it.
func (configuration Configuration) MarshalJSON() ([]byte, error) {
	if err := validateConfiguration(configuration); err != nil {
		return nil, err
	}
	return json.Marshal(configurationToJSON(configuration))
}

func (configuration *Configuration) UnmarshalJSON(data []byte) error {
	wire := configurationWire{}
	if err := decodeExactJSON(data, &wire); err != nil {
		return fmt.Errorf("decode version-2 configuration: %w", err)
	}
	decoded, err := configurationFromWire(wire)
	if err != nil {
		return err
	}
	*configuration = decoded
	return nil
}

func configurationToJSON(configuration Configuration) configurationJSON {
	return configurationJSON{
		Version:  configuration.Version,
		Settings: configuration.Settings,
		Test: savedTestJSON{
			Mode:            configuration.Test.Mode,
			Pack:            configuration.Test.Pack,
			DurationSeconds: configuration.Test.DurationSeconds,
			Count:           configuration.Test.Count,
			Modifiers: testModifiersJSON{
				Punctuation:    configuration.Test.Modifiers.Punctuation,
				Numbers:        configuration.Test.Modifiers.Numbers,
				Capitalization: configuration.Test.Modifiers.Capitalization,
			},
			Difficulty: configuration.Test.Difficulty,
		},
		Appearance:      configuration.Appearance,
		DiscoveredHints: configuration.DiscoveredHints,
	}
}

func configurationFromWire(wire configurationWire) (Configuration, error) {
	if wire.Version == nil || wire.Settings == nil || wire.Test == nil ||
		wire.Appearance == nil || wire.DiscoveredHints == nil {
		return Configuration{}, errors.New("configuration document is missing required fields")
	}
	if wire.Test.Mode == nil || wire.Test.Pack == nil || wire.Test.DurationSeconds == nil ||
		wire.Test.Count == nil || wire.Test.Modifiers == nil || wire.Test.Difficulty == nil {
		return Configuration{}, errors.New("test configuration is missing required fields")
	}
	if wire.Test.Modifiers.Punctuation == nil || wire.Test.Modifiers.Numbers == nil ||
		wire.Test.Modifiers.Capitalization == nil {
		return Configuration{}, errors.New("test modifiers are missing required fields")
	}
	appearance := wire.Appearance
	if appearance.Theme == nil || appearance.Focus == nil || appearance.ShowErrors == nil ||
		appearance.ReducedMotion == nil || appearance.KeySound == nil || appearance.ErrorSound == nil ||
		appearance.CompletionSound == nil || appearance.PBSound == nil {
		return Configuration{}, errors.New("appearance configuration is missing required fields")
	}
	settings, err := runtimeSettingsFromWire(wire.Settings)
	if err != nil {
		return Configuration{}, err
	}
	configuration := Configuration{
		Version:  *wire.Version,
		Settings: settings,
		Test: SavedTest{
			Mode:            *wire.Test.Mode,
			Pack:            *wire.Test.Pack,
			DurationSeconds: *wire.Test.DurationSeconds,
			Count:           *wire.Test.Count,
			Modifiers: TestModifiers{
				Punctuation:    *wire.Test.Modifiers.Punctuation,
				Numbers:        *wire.Test.Modifiers.Numbers,
				Capitalization: *wire.Test.Modifiers.Capitalization,
			},
			Difficulty: *wire.Test.Difficulty,
		},
		Appearance: Appearance{
			Theme:           *appearance.Theme,
			Focus:           *appearance.Focus,
			ShowErrors:      *appearance.ShowErrors,
			ReducedMotion:   *appearance.ReducedMotion,
			KeySound:        *appearance.KeySound,
			ErrorSound:      *appearance.ErrorSound,
			CompletionSound: *appearance.CompletionSound,
			PBSound:         *appearance.PBSound,
		},
		DiscoveredHints: append([]string(nil), (*wire.DiscoveredHints)...),
	}
	if configuration.DiscoveredHints == nil {
		configuration.DiscoveredHints = []string{}
	}
	if err := validateConfiguration(configuration); err != nil {
		return Configuration{}, err
	}
	return configuration, nil
}

func runtimeSettingsFromWire(wire *runtimeSettingsWire) (runtimeSettings, error) {
	if wire == nil || wire.ShowWPM == nil || wire.SkipWord == nil ||
		wire.AllowBackspace == nil || wire.BlockCursor == nil ||
		wire.BoldTypedText == nil || wire.Highlight == nil {
		return runtimeSettings{}, errors.New("settings document is missing required fields")
	}
	settings := runtimeSettings{
		ShowWPM:        *wire.ShowWPM,
		SkipWord:       *wire.SkipWord,
		AllowBackspace: *wire.AllowBackspace,
		BlockCursor:    *wire.BlockCursor,
		BoldTypedText:  *wire.BoldTypedText,
		Highlight:      *wire.Highlight,
	}
	if !validHighlightMode(settings.Highlight) {
		return runtimeSettings{}, fmt.Errorf("invalid highlight mode %q", settings.Highlight)
	}
	return settings, nil
}

func validateConfiguration(configuration Configuration) error {
	if configuration.Version != configurationVersion {
		return fmt.Errorf("unsupported settings version %d", configuration.Version)
	}
	if !validHighlightMode(configuration.Settings.Highlight) {
		return fmt.Errorf("invalid highlight mode %q", configuration.Settings.Highlight)
	}
	if err := ValidateSavedTest(configuration.Test); err != nil {
		return fmt.Errorf("invalid saved test: %w", err)
	}
	if !validConfigurationIdentifier(configuration.Appearance.Theme) {
		return fmt.Errorf("invalid appearance theme %q", configuration.Appearance.Theme)
	}
	if err := validateOptionalIdentifier("keySound", configuration.Appearance.KeySound); err != nil {
		return err
	}
	if err := validateOptionalIdentifier("errorSound", configuration.Appearance.ErrorSound); err != nil {
		return err
	}
	if err := validateOptionalIdentifier("completionSound", configuration.Appearance.CompletionSound); err != nil {
		return err
	}
	if err := validateOptionalIdentifier("pbSound", configuration.Appearance.PBSound); err != nil {
		return err
	}
	if configuration.DiscoveredHints == nil {
		return errors.New("discoveredHints must be an array")
	}
	return nil
}

func validateOptionalIdentifier(name, value string) error {
	if value != "" && !validConfigurationIdentifier(value) {
		return fmt.Errorf("invalid appearance %s %q", name, value)
	}
	return nil
}

// ValidateSavedTest validates the persisted TUI test boundary. Legacy CLI
// inputs are intentionally not subject to these custom-length limits.
func ValidateSavedTest(test SavedTest) error {
	if test.Difficulty != "normal" {
		return fmt.Errorf("unsupported difficulty %q", test.Difficulty)
	}
	resourceType := "words"
	switch test.Mode {
	case "timed":
		if test.DurationSeconds < 5 || test.DurationSeconds > 3600 {
			return fmt.Errorf("duration must be between 5 and 3600 seconds")
		}
	case "count":
		if test.Count < 1 || test.Count > 500 {
			return fmt.Errorf("count must be between 1 and 500 words")
		}
	case "quote":
		resourceType = "quotes"
	default:
		return fmt.Errorf("unsupported test mode %q", test.Mode)
	}
	if !validConfigurationIdentifier(test.Pack) {
		return fmt.Errorf("invalid %s pack %q", resourceType, test.Pack)
	}
	if _, ok := packedFiles[resourceType+"/"+test.Pack]; !ok {
		return fmt.Errorf("unknown %s pack %q", resourceType, test.Pack)
	}
	return nil
}

func validConfigurationIdentifier(value string) bool {
	if value == "" || value == "." || value == ".." || filepath.IsAbs(value) {
		return false
	}
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character >= '0' && character <= '9':
		case character == '-', character == '_':
		default:
			return false
		}
	}
	return true
}

// LoadConfiguration loads version 2, safely migrates a valid version-1 file,
// and returns defaults only when the file is missing. Invalid or unknown files
// are preserved in place and reported to the caller.
func LoadConfiguration(path string) (Configuration, error) {
	configuration, err := loadExistingConfiguration(path)
	if err == nil {
		return configuration, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return DefaultConfiguration(), nil
	}
	return DefaultConfiguration(), configurationRecoveryError(path, err)
}

func loadExistingConfiguration(path string) (Configuration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Configuration{}, fmt.Errorf("read configuration: %w", err)
	}
	version, err := documentVersion(data)
	if err != nil {
		return Configuration{}, err
	}
	switch version {
	case configurationVersion:
		return decodeVersion2(data)
	case 1:
		return migrateVersion1(path)
	default:
		return Configuration{}, fmt.Errorf("unsupported settings version %d", version)
	}
}

func documentVersion(data []byte) (int, error) {
	var header struct {
		Version *int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return 0, fmt.Errorf("decode configuration header: %w", err)
	}
	if header.Version == nil {
		return 0, errors.New("configuration document is missing version")
	}
	return *header.Version, nil
}

func decodeVersion2(data []byte) (Configuration, error) {
	var configuration Configuration
	if err := json.Unmarshal(data, &configuration); err != nil {
		return Configuration{}, err
	}
	return configuration, nil
}

func decodeVersion1(data []byte) (runtimeSettings, error) {
	wire := legacyConfigurationWire{}
	if err := decodeExactJSON(data, &wire); err != nil {
		return runtimeSettings{}, fmt.Errorf("decode version-1 settings: %w", err)
	}
	if wire.Version == nil || *wire.Version != 1 {
		return runtimeSettings{}, errors.New("not a version-1 settings document")
	}
	settings, err := runtimeSettingsFromWire(wire.Settings)
	if err != nil {
		return runtimeSettings{}, err
	}
	return settings, nil
}

func decodeExactJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func configurationRecoveryError(path string, err error) error {
	return fmt.Errorf("load configuration %q: %w; original preserved, fix or move the file to recover", path, err)
}

func migrateVersion1(path string) (Configuration, error) {
	var migrated Configuration
	err := withConfigurationLock(path, func() error {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				migrated = DefaultConfiguration()
				return nil
			}
			return fmt.Errorf("reread version-1 settings: %w", err)
		}
		version, err := documentVersion(data)
		if err != nil {
			return err
		}
		if version == configurationVersion {
			migrated, err = decodeVersion2(data)
			return err
		}
		if version != 1 {
			return fmt.Errorf("unsupported settings version %d", version)
		}
		settings, err := decodeVersion1(data)
		if err != nil {
			return err
		}
		migrated = DefaultConfiguration()
		migrated.Settings = settings
		if _, err := writeVerifiedVersion1Backup(path, data); err != nil {
			return fmt.Errorf("back up version-1 settings: %w", err)
		}
		encoded, err := marshalConfiguration(migrated)
		if err != nil {
			return err
		}
		if err := writeConfigurationAtomically(path, encoded); err != nil {
			return fmt.Errorf("commit migrated configuration: %w", err)
		}
		return nil
	})
	if err != nil {
		return Configuration{}, fmt.Errorf("migrate version-1 settings: %w", err)
	}
	return migrated, nil
}

// CommitConfiguration serializes all config writers through one lock, reloads
// the latest valid document, and applies only the named dirty paths.
func CommitConfiguration(path string, draft Configuration, dirty []string) error {
	_, err := commitConfiguration(path, draft, dirty)
	return err
}

func commitConfiguration(path string, draft Configuration, dirty []string) (Configuration, error) {
	if err := validateConfiguration(draft); err != nil {
		return Configuration{}, fmt.Errorf("validate configuration draft: %w", err)
	}
	if err := validateDirtyPaths(dirty); err != nil {
		return Configuration{}, err
	}
	if len(dirty) == 0 {
		return draft, nil
	}

	var committed Configuration
	err := withConfigurationLock(path, func() error {
		current, legacy, err := readConfigurationForCommit(path)
		if err != nil {
			return err
		}
		committed = current
		for _, path := range dirty {
			applyDirtyPath(&committed, draft, path)
		}
		committed.Version = configurationVersion
		if err := validateConfiguration(committed); err != nil {
			return fmt.Errorf("validate merged configuration: %w", err)
		}
		encoded, err := marshalConfiguration(committed)
		if err != nil {
			return err
		}
		if legacy != nil {
			if _, err := writeVerifiedVersion1Backup(path, legacy); err != nil {
				return fmt.Errorf("back up version-1 settings: %w", err)
			}
		}
		if err := writeConfigurationAtomically(path, encoded); err != nil {
			return fmt.Errorf("commit configuration: %w", err)
		}
		return nil
	})
	if err != nil {
		return Configuration{}, fmt.Errorf("save configuration %q: %w", path, err)
	}
	return committed, nil
}

func readConfigurationForCommit(path string) (Configuration, []byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultConfiguration(), nil, nil
	}
	if err != nil {
		return Configuration{}, nil, fmt.Errorf("read current configuration: %w", err)
	}
	version, err := documentVersion(data)
	if err != nil {
		return Configuration{}, nil, err
	}
	switch version {
	case configurationVersion:
		configuration, err := decodeVersion2(data)
		return configuration, nil, err
	case 1:
		settings, err := decodeVersion1(data)
		if err != nil {
			return Configuration{}, nil, err
		}
		configuration := DefaultConfiguration()
		configuration.Settings = settings
		return configuration, data, nil
	default:
		return Configuration{}, nil, fmt.Errorf("unsupported settings version %d", version)
	}
}

func validateDirtyPaths(paths []string) error {
	for _, path := range paths {
		switch path {
		case "settings", "settings.showWPM", "settings.skipWord", "settings.allowBackspace",
			"settings.blockCursor", "settings.boldTypedText", "settings.highlight",
			"test", "test.mode", "test.pack", "test.durationSeconds", "test.count",
			"test.modifiers", "test.modifiers.punctuation", "test.modifiers.numbers",
			"test.modifiers.capitalization", "test.difficulty",
			"appearance", "appearance.theme", "appearance.focus", "appearance.showErrors",
			"appearance.reducedMotion", "appearance.keySound", "appearance.errorSound",
			"appearance.completionSound", "appearance.pbSound", "discoveredHints":
		default:
			return fmt.Errorf("unknown dirty configuration path %q", path)
		}
	}
	return nil
}

func applyDirtyPath(configuration *Configuration, draft Configuration, path string) {
	switch path {
	case "settings":
		configuration.Settings = draft.Settings
	case "settings.showWPM":
		configuration.Settings.ShowWPM = draft.Settings.ShowWPM
	case "settings.skipWord":
		configuration.Settings.SkipWord = draft.Settings.SkipWord
	case "settings.allowBackspace":
		configuration.Settings.AllowBackspace = draft.Settings.AllowBackspace
	case "settings.blockCursor":
		configuration.Settings.BlockCursor = draft.Settings.BlockCursor
	case "settings.boldTypedText":
		configuration.Settings.BoldTypedText = draft.Settings.BoldTypedText
	case "settings.highlight":
		configuration.Settings.Highlight = draft.Settings.Highlight
	case "test":
		configuration.Test = draft.Test
	case "test.mode":
		configuration.Test.Mode = draft.Test.Mode
	case "test.pack":
		configuration.Test.Pack = draft.Test.Pack
	case "test.durationSeconds":
		configuration.Test.DurationSeconds = draft.Test.DurationSeconds
	case "test.count":
		configuration.Test.Count = draft.Test.Count
	case "test.modifiers":
		configuration.Test.Modifiers = draft.Test.Modifiers
	case "test.modifiers.punctuation":
		configuration.Test.Modifiers.Punctuation = draft.Test.Modifiers.Punctuation
	case "test.modifiers.numbers":
		configuration.Test.Modifiers.Numbers = draft.Test.Modifiers.Numbers
	case "test.modifiers.capitalization":
		configuration.Test.Modifiers.Capitalization = draft.Test.Modifiers.Capitalization
	case "test.difficulty":
		configuration.Test.Difficulty = draft.Test.Difficulty
	case "appearance":
		configuration.Appearance = draft.Appearance
	case "appearance.theme":
		configuration.Appearance.Theme = draft.Appearance.Theme
	case "appearance.focus":
		configuration.Appearance.Focus = draft.Appearance.Focus
	case "appearance.showErrors":
		configuration.Appearance.ShowErrors = draft.Appearance.ShowErrors
	case "appearance.reducedMotion":
		configuration.Appearance.ReducedMotion = draft.Appearance.ReducedMotion
	case "appearance.keySound":
		configuration.Appearance.KeySound = draft.Appearance.KeySound
	case "appearance.errorSound":
		configuration.Appearance.ErrorSound = draft.Appearance.ErrorSound
	case "appearance.completionSound":
		configuration.Appearance.CompletionSound = draft.Appearance.CompletionSound
	case "appearance.pbSound":
		configuration.Appearance.PBSound = draft.Appearance.PBSound
	case "discoveredHints":
		configuration.DiscoveredHints = append([]string{}, draft.DiscoveredHints...)
	}
}

func marshalConfiguration(configuration Configuration) ([]byte, error) {
	if err := validateConfiguration(configuration); err != nil {
		return nil, fmt.Errorf("validate configuration: %w", err)
	}
	data, err := json.MarshalIndent(configurationToJSON(configuration), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode configuration: %w", err)
	}
	return append(data, '\n'), nil
}

func withConfigurationLock(path string, action func() error) error {
	directory := filepath.Dir(path)
	if err := ensurePrivateDirectory(directory); err != nil {
		return fmt.Errorf("prepare configuration directory: %w", err)
	}
	lockPath := path + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open configuration lock: %w", err)
	}
	defer lock.Close()
	if err := lock.Chmod(0600); err != nil {
		return fmt.Errorf("secure configuration lock: %w", err)
	}
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("acquire configuration lock: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return action()
}

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	return os.Chmod(path, 0700)
}

func writeConfigurationAtomically(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := ensurePrivateDirectory(directory); err != nil {
		return fmt.Errorf("prepare destination directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".settings-*")
	if err != nil {
		return fmt.Errorf("create temporary configuration: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary configuration: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary configuration: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary configuration: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary configuration: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace configuration: %w", err)
	}
	removeTemporary = false
	// Rename is the commit point. A directory-sync failure cannot be reported
	// as a failed save after the previous file has already been replaced.
	_ = syncDirectory(directory)
	return nil
}

func writeVerifiedVersion1Backup(path string, original []byte) (string, error) {
	backupDirectory := filepath.Join(filepath.Dir(path), "backups")
	if err := ensurePrivateDirectory(backupDirectory); err != nil {
		return "", fmt.Errorf("prepare backup directory: %w", err)
	}
	identifierBytes := make([]byte, 16)
	if _, err := rand.Read(identifierBytes); err != nil {
		return "", fmt.Errorf("generate backup identifier: %w", err)
	}
	backupPath := filepath.Join(backupDirectory, "settings-v1-"+hex.EncodeToString(identifierBytes)+".json")
	backup, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("create backup: %w", err)
	}
	verified := false
	defer func() {
		_ = backup.Close()
		if !verified {
			_ = os.Remove(backupPath)
		}
	}()
	if err := backup.Chmod(0600); err != nil {
		return "", fmt.Errorf("secure backup: %w", err)
	}
	if _, err := backup.Write(original); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}
	if err := backup.Sync(); err != nil {
		return "", fmt.Errorf("sync backup: %w", err)
	}
	if err := backup.Close(); err != nil {
		return "", fmt.Errorf("close backup: %w", err)
	}
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return "", fmt.Errorf("verify backup contents: %w", err)
	}
	if sha256.Sum256(backupData) != sha256.Sum256(original) {
		return "", errors.New("backup hash verification failed")
	}
	info, err := os.Stat(backupPath)
	if err != nil {
		return "", fmt.Errorf("verify backup mode: %w", err)
	}
	if info.Mode().Perm() != 0600 {
		return "", fmt.Errorf("backup mode is %04o, want 0600", info.Mode().Perm())
	}
	if err := syncDirectory(backupDirectory); err != nil {
		return "", fmt.Errorf("sync backup directory: %w", err)
	}
	verified = true
	return backupPath, nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	err = directory.Sync()
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) {
		return nil
	}
	return err
}
