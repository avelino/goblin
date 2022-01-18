package resolver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver"
)

const DEFAULT_PROXY_URL = "https://proxy.golang.org"

const hashRegex = "^[0-9a-f]{7,40}$"

type Resolver struct {
	Pkg             string
	Value           string
	Hash            bool
	ConstraintCheck *semver.Constraints
}

type VersionInfo struct {
	Version string    // version string
	Time    time.Time // commit time
}

// Resolve the version for the given package by
// checking with the proxy for either the specified version
// or getting the latest version on the proxy
func (v *Resolver) ResolveVersion() (string, error) {
	if len(v.Value) == 0 {
		version, err := v.ResolveLatestVersion()
		return version.Version, err
	}

	if v.Hash {
		return v.Value, nil
	}

	return v.ResolveClosestVersion()
}

// resolve the latest version from the proxy
func (v *Resolver) ResolveLatestVersion() (VersionInfo, error) {
	var versionInfo VersionInfo

	resp, err := http.Get(getVersionLatestProxyURL(v.Pkg))
	if err != nil {
		return versionInfo, err
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return versionInfo, err
	}

	if err := json.Unmarshal(respBytes, &versionInfo); err != nil {
		return versionInfo, err
	}

	return versionInfo, nil
}

// Parse the given string to be either a semver version string
// or a commit hash
func (v *Resolver) ParseVersion(version string) error {
	v.Value = version

	// just send back if no version is provided
	if len(version) == 0 {
		return nil
	}

	versionConstraint, err := isSemver(version)

	// return the string back if it's a valid hash string
	if err != nil {
		matched, err := regexp.MatchString(hashRegex, version)
		if matched {
			v.Hash = true
			return nil
		}
		// if not a hash or a semver, just return an error
		if err != nil {
			return err
		}
	}
	v.ConstraintCheck = versionConstraint
	return nil
}

// Resolve the closes version to the given semver from the proxy
func (v *Resolver) ResolveClosestVersion() (string, error) {
	var versionTags []string

	resp, err := http.Get(getVersionListProxyURL(v.Pkg))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	versionTags = strings.Split(string(data), "\n")
	matchedVersion := ""

	for _, versionTag := range versionTags {
		if len(versionTag) == 0 {
			continue
		}

		ver, err := semver.NewVersion(versionTag)
		if err != nil {
			return "", err
		}

		if !v.ConstraintCheck.Check(
			ver,
		) {
			continue
		}

		matchedVersion = versionTag
	}

	if len(matchedVersion) == 0 {
		return "", nil
	}

	return matchedVersion, nil
}

// check if the given string is valid semver string and if yest
// create a constraint checker out of it
func isSemver(version string) (*semver.Constraints, error) {
	_, err := semver.NewVersion(version)
	if err != nil {
		return nil, err
	}

	return semver.NewConstraint("= " + version)
}

// normalize the proxy url to
// - not have traling slashes
func normalizeUrl(url string) string {
	if strings.HasSuffix(url, "/") {
		ind := strings.LastIndex(url, "/")
		if ind == -1 {
			return url
		}
		return strings.Join([]string{url[:ind], "", url[ind+1:]}, "")
	}
	return url
}

// get the proxy url for the latest version
func getVersionLatestProxyURL(pkg string) string {
	urlPrefix := normalizeUrl(DEFAULT_PROXY_URL)
	return urlPrefix + "/" + pkg + "/@latest"
}

// get the proxy url for the entire version list
func getVersionListProxyURL(pkg string) string {
	urlPrefix := normalizeUrl(DEFAULT_PROXY_URL)
	return urlPrefix + "/" + pkg + "/@v/list"
}
