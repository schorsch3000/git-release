package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

const ConfigFile = ".git-release.toml"

// Config is the content of .git-release.toml.
type Config struct {
	Branch    string   `toml:"branch"`
	TagPrefix *string  `toml:"tag_prefix"`
	Sign      bool     `toml:"sign"`
	Annotated *bool    `toml:"annotated"`
	Pre       []string `toml:"pre"`
	Bump      []string `toml:"bump"`
	Post      []string `toml:"post"`
}

func (c Config) annotated() bool { return c.Annotated == nil || *c.Annotated }

// LoadConfig reads the config from root. A missing file yields defaults and exists=false.
func LoadConfig(root string) (cfg Config, exists bool, err error) {
	path := filepath.Join(root, ConfigFile)
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		return cfg, false, nil
	}
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return cfg, true, fmt.Errorf("%s: %w", ConfigFile, err)
	}
	if un := md.Undecoded(); len(un) > 0 {
		keys := make([]string, len(un))
		for i, k := range un {
			keys[i] = k.String()
		}
		return cfg, true, fmt.Errorf("%s: unknown key(s): %s", ConfigFile, strings.Join(keys, ", "))
	}
	if cfg.TagPrefix != nil && *cfg.TagPrefix != "" && *cfg.TagPrefix != "v" {
		return cfg, true, fmt.Errorf(`%s: tag_prefix must be "v" or ""`, ConfigFile)
	}
	return cfg, true, nil
}

var tagPrefixLine = regexp.MustCompile(`(?m)^[ \t]*tag_prefix[ \t]*=.*$`)

// SaveTagPrefix sets tag_prefix in the config file, creating it if needed.
func SaveTagPrefix(root, prefix string) error {
	path := filepath.Join(root, ConfigFile)
	line := fmt.Sprintf("tag_prefix = %q", prefix)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return os.WriteFile(path, []byte(line+"\n"), 0o644)
	}
	if err != nil {
		return err
	}
	if tagPrefixLine.Match(b) {
		b = tagPrefixLine.ReplaceAll(b, []byte(line))
	} else {
		s := string(b)
		if s != "" && !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		b = []byte(s + line + "\n")
	}
	return os.WriteFile(path, b, 0o644)
}

const starterConfig = `# git-release configuration. All keys are optional.

# Release branch. Default: the remote's default branch.
# branch = "main"

# Tag style: "v" (v1.2.3) or "" (1.2.3). Auto-detected from existing tags.
# tag_prefix = "v"

# Create a signed tag.
# sign = false

# Checks. Run in order, stop at the first failure. Must not modify the worktree.
# pre = ["make test", "make lint"]

# File changes for the release. Tracked-file changes are committed as "release: <tag>".
# Environment: GIT_RELEASE_VERSION, GIT_RELEASE_TAG, GIT_RELEASE_PREVIOUS, GIT_RELEASE_BRANCH
# bump = [
#   "jq '.version=\"$GIT_RELEASE_VERSION\"' package.json > package.json.tmp && mv package.json.tmp package.json",
# ]

# Runs after the tag exists. Nothing is pushed unless you do it here.
# post = ["git push origin HEAD --follow-tags"]
`
