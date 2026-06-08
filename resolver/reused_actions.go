package resolver

import (
	"fmt"
	"os"
)

// ReusedAction is a unique external action reference enriched with repository metadata.
type ReusedAction struct {
	Uses         string `json:"uses"`
	Contributors int    `json:"contributors"`
	Stars        int    `json:"stars"`
	ReleasedOn   string `json:"released_on,omitempty"`
}

type repoStats struct {
	stars        int
	contributors int
}

// EnrichReusedActions fetches GitHub metadata for each action reference.
// Repo-level stats (stars, contributors) are cached by owner/repo.
func EnrichReusedActions(client *GitHubClient, refs []ResolvedActionRef) ([]ReusedAction, error) {
	cache := make(map[string]repoStats)
	result := make([]ReusedAction, 0, len(refs))

	for _, item := range refs {
		ref := item.Ref
		fmt.Fprintf(os.Stderr, "  Fetching metadata for %s...\n", ref.RawUses)

		repoKey := ref.Owner + "/" + ref.Repo
		stats, ok := cache[repoKey]
		if !ok {
			stars, err := client.GetRepoStars(ref.Owner, ref.Repo)
			if err != nil {
				fmt.Fprintf(os.Stderr, "    Warning: stars for %s: %v\n", repoKey, err)
			}
			contribs, err := client.CountContributors(ref.Owner, ref.Repo)
			if err != nil {
				fmt.Fprintf(os.Stderr, "    Warning: contributors for %s: %v\n", repoKey, err)
			}
			stats = repoStats{stars: stars, contributors: contribs}
			cache[repoKey] = stats
		}

		releasedOn := ""
		if released, err := client.GetReleasedOn(ref.Owner, ref.Repo, ref.Ref, item.SHA); err == nil {
			releasedOn = released
		} else {
			fmt.Fprintf(os.Stderr, "    Warning: released_on for %s@%s: %v\n", ref.FullName(), ref.Ref, err)
		}

		result = append(result, ReusedAction{
			Uses:         ref.RawUses,
			Contributors: stats.contributors,
			Stars:        stats.stars,
			ReleasedOn:   releasedOn,
		})
	}

	return result, nil
}
