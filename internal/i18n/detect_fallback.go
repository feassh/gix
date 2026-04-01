//go:build !darwin && !linux && !windows

package i18n

func detectPlatformLocales() []string {
	return nil
}
