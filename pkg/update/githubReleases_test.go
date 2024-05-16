package update

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chainguard.dev/melange/pkg/config"

	"github.com/wolfi-dev/wolfictl/pkg/melange"

	"github.com/stretchr/testify/assert"
)

func TestMonitorService_parseGitHubReleases(t *testing.T) {
	packageConfigs := make(map[string]*melange.Packages)

	packageConfigs["cosign"] = &melange.Packages{
		Config: config.Configuration{
			Package: config.Package{Name: "cosign", Version: "1.10.1"},
			Update: config.Update{GitHubMonitor: &config.GitHubMonitor{
				Identifier:  "sigstore/cosign",
				StripPrefix: "v",
			}},
		},
	}

	packageConfigs["jenkins"] = &melange.Packages{
		Config: config.Configuration{
			Package: config.Package{Name: "jenkins", Version: "2.370"},
			Update: config.Update{GitHubMonitor: &config.GitHubMonitor{
				Identifier:  "jenkinsci/jenkins",
				StripPrefix: "jenkins-",
			}},
		},
	}
	tests := []struct {
		name            string
		packageName     string
		initialVersion  string
		expectedVersion string
		githubMonitor   config.GitHubMonitor
	}{
		{
			name:            "multiple_repos",
			packageName:     "cosign",
			expectedVersion: "2.0.0",
		},
		{
			name:            "multiple_repos",
			packageName:     "jenkins",
			expectedVersion: "2.397",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", test.name, "graphql_versions_results.json"))
			assert.NoError(t, err)
			assert.NotEmpty(t, data)

			m := NewGitHubReleaseOptions(packageConfigs, nil)

			rel := QueryReleasesResponse{}
			err = json.Unmarshal(data, &rel)
			assert.NoError(t, err)
			assert.NotEmpty(t, rel)

			rel.Data = getReleaseResultsMapWithHashKeys(rel, m)

			errorMessages := make(map[string]string)

			latestVersions, err := m.parseGitHubReleases(&rel)
			assert.NoError(t, err)
			assert.Empty(t, errorMessages)
			assert.Equal(t, test.expectedVersion, latestVersions[test.packageName].Version)
		})
	}
}

func getReleaseResultsMapWithHashKeys(rel QueryReleasesResponse, m GitHubReleaseOptions) map[string]Releases {
	// modify the fake github response with the hash keys generated from NewGitHubReleaseOptions
	// this is so we can match the response back with the melange package
	copyMap := make(map[string]Releases)
	for old, config := range rel.Data {
		for s, packages := range m.PackageConfigs {
			if packages.Config.Update.GitHubMonitor.Identifier == config.NameWithOwner {
				copyMap[m.PackageConfigs[s].Hash] = config
				delete(rel.Data, old)
				continue
			}
		}
	}
	return copyMap
}
func getTagResultsMapWithHashKeys(rel QueryTagsResponse, m GitHubReleaseOptions) map[string]Repo {
	// modify the fake github response with the hash keys generated from NewGitHubReleaseOptions
	// this is so we can match the response back with the melange package
	copyMap := make(map[string]Repo)
	for old, config := range rel.Data {
		for s, packages := range m.PackageConfigs {
			if packages.Config.Update.GitHubMonitor.Identifier == config.NameWithOwner {
				copyMap[m.PackageConfigs[s].Hash] = config
				delete(rel.Data, old)
				continue
			}
		}
	}
	return copyMap
}

func TestMonitorService_parseGitHubTags(t *testing.T) {
	tests := []struct {
		name            string
		packageName     string
		expectedVersion string
		updateConfig    config.Configuration
	}{
		{
			name:            "parse_go_tags",
			packageName:     "go-1.19",
			expectedVersion: "1.19.7",
			updateConfig: config.Configuration{
				Package: config.Package{Name: "go-1.19", Version: "1.19.1"},
				Update: config.Update{
					Enabled: true,
					GitHubMonitor: &config.GitHubMonitor{
						Identifier:  "golang/go",
						TagFilter:   "go1.19",
						StripPrefix: "go",
					},
				},
			},
		},
		{
			name:            "parse_java_tags",
			packageName:     "openjdk-11",
			expectedVersion: "11.0.19+5",
			updateConfig: config.Configuration{
				Package: config.Package{Name: "openjdk-11", Version: "11.0.16"},
				Update: config.Update{
					Enabled: true,
					GitHubMonitor: &config.GitHubMonitor{
						Identifier:  "openjdk/jdk11u",
						TagFilter:   "jdk-11",
						StripPrefix: "jdk-",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", test.name, "graphql_versions_results.json"))
			assert.NoError(t, err)
			assert.NotEmpty(t, data)

			packageConfigs := make(map[string]*melange.Packages)

			packageConfigs[test.packageName] = &melange.Packages{
				Config: test.updateConfig,
			}

			m := NewGitHubReleaseOptions(packageConfigs, nil)

			var rel QueryTagsResponse
			err = json.Unmarshal(data, &rel)
			assert.NoError(t, err)
			assert.NotEmpty(t, rel)

			rel.Data = getTagResultsMapWithHashKeys(rel, m)

			errorMessages := make(map[string]string)
			latestVersions, err := m.parseGitHubTags(&rel)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedVersion, latestVersions[test.packageName].Version)
			assert.Empty(t, errorMessages)
		})
	}
}

func Test_queryTemplate(t *testing.T) {
	data := QueryTagsData{
		RepoList: []RepoQuery{
			{Owner: "golang", Name: "go", Filter: "go1.19"},
			{Owner: "openjdk", Name: "jdk11u", Filter: "jdk-11"},
		},
	}

	got := template(queryTags, data)
	assert.NotEmpty(t, got)

	expected, err := os.ReadFile(filepath.Join("testdata", "query_tags", "result"))
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	assert.Equal(t, strings.TrimSpace(string(expected)), strings.TrimSpace(got))
}

func Test_queryTagsResult(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "git_tags", "graphql_versions_results.json"))
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	var response QueryTagsResponse
	err = json.Unmarshal(data, &response)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(response.Data))
	assert.Contains(t, "golang/go", response.Data["repo_0"].NameWithOwner)
	assert.Contains(t, "openjdk/jdk11u", response.Data["repo_1"].NameWithOwner)
}

func TestGitHubReleaseOptions_isVersionPreRelease(t *testing.T) {
	tests := []struct {
		version string
		skip    bool
	}{
		{version: "1.2.3-alpha", skip: true},
		{version: "1.2.3-beta", skip: true},
		{version: "1.2.3-rc", skip: true},
		{version: "1.2.3", skip: false},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			o := GitHubReleaseOptions{
				Logger: log.New(log.Writer(), "test: ", log.LstdFlags|log.Lmsgprefix),
			}

			assert.Equalf(t, tt.skip, o.shouldSkipVersion(tt.version), "isVersionPreRelease(%v)", tt.version)
		})
	}
}

func Test_validSha(t *testing.T) {
	tests := []struct {
		name     string
		objIDstr string
		wantErr  assert.ErrorAssertionFunc
	}{
		{name: "simple", objIDstr: "7bd22aafe41be40e2174335a3dc55431ca9548ec", wantErr: assert.NoError},
		{name: "bad_sha", objIDstr: "abc123", wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validSha(tt.objIDstr)
			if !tt.wantErr(t, err, fmt.Sprintf("valid(%v)", tt.objIDstr)) {
				return
			}
		})
	}
}

func TestGitHubReleaseOptions_prepareVersion(t *testing.T) {
	tests := []struct {
		name          string
		melangeConfig config.Configuration
		version       string
		want          string
		wantErr       assert.ErrorAssertionFunc
	}{
		{name: "regex", melangeConfig: config.Configuration{
			Update: config.Update{
				IgnoreRegexPatterns: []string{"dec*"},
				GitHubMonitor:       &config.GitHubMonitor{},
			},
		}, version: "1.2.3dec11", want: "", wantErr: assert.NoError},
		{name: "regex_error", melangeConfig: config.Configuration{
			Update: config.Update{
				IgnoreRegexPatterns: []string{"ab(c"},
				GitHubMonitor:       &config.GitHubMonitor{},
			},
		}, version: "1.2.3dec11", want: "", wantErr: assert.Error},
		{name: "strip_prefix", melangeConfig: config.Configuration{
			Update: config.Update{
				GitHubMonitor: &config.GitHubMonitor{
					StripPrefix: "v",
				},
			},
		}, version: "v1.2.3", want: "1.2.3", wantErr: assert.NoError},
		{name: "strip_suffix", melangeConfig: config.Configuration{
			Update: config.Update{
				GitHubMonitor: &config.GitHubMonitor{
					StripSuffix: "blah",
				},
			},
		}, version: "1.2.3blah", want: "1.2.3", wantErr: assert.NoError},
		{name: "tag-filter", melangeConfig: config.Configuration{
			Update: config.Update{
				GitHubMonitor: &config.GitHubMonitor{
					TagFilter: "v",
				},
			},
		}, version: "1.2.3", want: "", wantErr: assert.NoError},
		{name: "tag-filter", melangeConfig: config.Configuration{
			Update: config.Update{
				GitHubMonitor: &config.GitHubMonitor{
					TagFilter: "v",
				},
			},
		}, version: "v1.2.3", want: "v1.2.3", wantErr: assert.NoError},
		{name: "tag-filter-contains", melangeConfig: config.Configuration{
			Update: config.Update{
				GitHubMonitor: &config.GitHubMonitor{
					TagFilterContains: "-ga",
					StripSuffix:       "-ga",
				},
			},
		}, version: "1.2.3-ga", want: "1.2.3", wantErr: assert.NoError},
		{name: "tag-filter-contains", melangeConfig: config.Configuration{
			Update: config.Update{
				GitHubMonitor: &config.GitHubMonitor{
					TagFilterContains: "-ga",
					TagFilterPrefix:   "v",
					StripSuffix:       "-ga",
					StripPrefix:       "v",
				},
			},
		}, version: "v1.2.3-ga", want: "1.2.3", wantErr: assert.NoError},
		{name: "tag-filter-prefix", melangeConfig: config.Configuration{
			Update: config.Update{
				GitHubMonitor: &config.GitHubMonitor{
					TagFilterPrefix: "v1.",
					StripSuffix:     "-ga",
				},
			},
		}, version: "v1.2.3-ga", want: "v1.2.3", wantErr: assert.NoError},
		{name: "transform-version", melangeConfig: config.Configuration{
			Update: config.Update{
				VersionTransform: []config.VersionTransform{
					{Match: "_", Replace: "."},
				},
				GitHubMonitor: &config.GitHubMonitor{},
			},
		}, version: "1_2_3", want: "1.2.3", wantErr: assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := GitHubReleaseOptions{}

			packageConfigs := make(map[string]*melange.Packages)
			configsByHash := make(map[string]config.Configuration)

			packageConfigs["foo"] = &melange.Packages{Config: tt.melangeConfig}
			packageConfigs["foo"].Hash = "bar"

			configsByHash["bar"] = tt.melangeConfig

			o.PackageConfigs = packageConfigs
			o.ConfigsByHash = configsByHash

			got, err := o.prepareVersion("bar", tt.version, "cheese/crisps")
			if !tt.wantErr(t, err, fmt.Sprintf("prepareVersion %s", tt.version)) {
				return
			}
			assert.Equalf(t, tt.want, got, "prepareVersion %s", tt.version)
		})
	}
}

func TestGitHubReleaseOptions_shouldIgnoreVersion(t *testing.T) {
	tests := []struct {
		name                string
		melangeConfig       config.Configuration
		version             string
		ignore              bool
		ignoreRegexPatterns []string
	}{
		{name: "ignore_odd_minor", ignoreRegexPatterns: []string{`(\d+)\.(\d*[13579])\.(\d+)$`}, version: "1.79.0", ignore: true},
		{name: "allow_even_minor", ignoreRegexPatterns: []string{`(\d+)\.(\d*[13579])\.(\d+)$`}, version: "1.80.0", ignore: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ignoreVersions(tt.ignoreRegexPatterns, tt.version)
			assert.NoError(t, err)
			assert.Equalf(t, tt.ignore, got, "shouldIgnoreVersion %s", tt.version)
		})
	}
}
