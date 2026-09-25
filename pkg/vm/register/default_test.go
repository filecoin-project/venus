package register

import (
	"fmt"
	"testing"

	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/network"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/actors"
)

// reachableActorVersions maps each actors version that go-state-types resolves a network
// version to, onto the first network version that reaches it. The versions come from the
// upstream mapping instead of venus's own generated table, so a version added upstream is
// covered by the checks below before venus vendors it.
func reachableActorVersions(t *testing.T) map[actorstypes.Version]network.Version {
	t.Helper()

	const scanLimit = network.Version(1024)
	reachable := map[actorstypes.Version]network.Version{}
	for nv := network.Version0; nv < scanLimit; nv++ {
		av, err := actorstypes.VersionForNetwork(nv)
		if err != nil {
			return reachable
		}
		if _, ok := reachable[av]; !ok {
			reachable[av] = nv
		}
	}

	t.Fatalf("go-state-types resolves every network version below %d: raise scanLimit", scanLimit)
	return nil
}

// assertBuiltinCodesResolve asserts that every builtin code cid venus knows for an actors
// version resolves through the default code loader. origin names the version set the
// assertion was reached from, so a failure points at the source that is out of step.
func assertBuiltinCodesResolve(t *testing.T, av actorstypes.Version, origin string) {
	t.Helper()

	codeIDs, err := actors.GetActorCodeIDs(av)
	if err != nil {
		t.Errorf("%s: cannot enumerate builtin code cids: %v", origin, err)
		return
	}
	if len(codeIDs) == 0 {
		t.Errorf("%s: no builtin code cids", origin)
		return
	}

	loader := GetDefaultActros()
	for key, code := range codeIDs {
		if _, err := loader.GetVMActor(code); err != nil {
			t.Errorf("%s (%s): builtin code %s does not resolve through GetDefaultActros: %v",
				origin, key, code, err)
		}
	}
}

// TestDefaultActrosResolvesEveryReachableActorVersion asserts that the default code loader
// resolves every builtin code of every actors version a chain can run: each version some
// network version resolves to, plus each version venus's generated table lists. A version
// missing from the loader leaves its actors unresolvable for the loader's consumers
// (legacy vm dispatcher, StateReadState RPC).
func TestDefaultActrosResolvesEveryReachableActorVersion(t *testing.T) {
	tf.UnitTest(t)

	reachable := reachableActorVersions(t)
	if len(reachable) == 0 {
		t.Fatal("go-state-types resolves no network version to an actors version")
	}

	for av, nv := range reachable {
		assertBuiltinCodesResolve(t, av, fmt.Sprintf("network v%d resolves to actors v%d", nv, av))
	}

	if len(actors.Versions) == 0 {
		t.Fatal("actors version table is empty")
	}

	// venus's generated table tracks go-state-types: every reachable version has to be listed,
	// and the newest listed version has to be the newest reachable one, so that an upstream
	// version cannot pass the gate while venus still runs with a stale table.
	newest := actorstypes.Version(-1)
	for av := range reachable {
		if av > newest {
			newest = av
		}
	}
	listed := make(map[actorstypes.Version]bool, len(actors.Versions))
	for _, v := range actors.Versions {
		listed[actorstypes.Version(v)] = true
	}
	for av, nv := range reachable {
		if !listed[av] {
			t.Errorf("network v%d resolves to actors v%d, which is missing from actors.Versions %v",
				nv, av, actors.Versions)
		}
	}
	if got := actorstypes.Version(actors.LatestVersion); got != newest {
		t.Errorf("actors.LatestVersion is %d but the newest version reachable from a network version is %d (network v%d)",
			got, newest, reachable[newest])
	}

	// Versions venus lists ahead of the upstream mapping have to be resolvable too.
	for _, v := range actors.Versions {
		av := actorstypes.Version(v)
		assertBuiltinCodesResolve(t, av, fmt.Sprintf("actors v%d", av))
	}
}
