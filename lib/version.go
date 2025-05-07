package beaver

import (
	"fmt"

	hv "github.com/hashicorp/go-version"
)

func ControlVersions(desired, actual string) error {
	desiredVersion, err := hv.NewVersion(desired)
	if err != nil {
		return fmt.Errorf("failed to parse desired beaver version: %w", err)
	}

	actualVersion, err := hv.NewVersion(actual)
	if err != nil {
		return fmt.Errorf("failed to parse actual beaver version: %w", err)
	}

	if !desiredVersion.Equal(actualVersion) {
		return fmt.Errorf(
			"desired beaver version is not equal to actual beaver version, %s != %s",
			desiredVersion.String(), actualVersion.String())
	}

	return nil
}
