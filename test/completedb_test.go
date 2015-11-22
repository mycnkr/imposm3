package test

import (
	"database/sql"

	"math"
	"testing"

	"github.com/omniscale/imposm3/geom/geos"
)

var ts importTestSuite

func TestPrepare(t *testing.T) {
	ts.dir = "/tmp/imposm3test"
	ts.config = importConfig{
		connection:      "postgis://",
		cacheDir:        ts.dir,
		osmFileName:     "build/complete_db.pbf",
		mappingFileName: "complete_db_mapping.json",
	}
	ts.g = geos.NewGeos()

	var err error
	ts.db, err = sql.Open("postgres", "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	ts.dropSchemas()
}

func TestImport(t *testing.T) {
	if ts.tableExists(t, dbschemaImport, "osm_roads") != false {
		t.Fatalf("table osm_roads exists in schema %s", dbschemaImport)
	}
	ts.importOsm(t)
	if ts.tableExists(t, dbschemaImport, "osm_roads") != true {
		t.Fatalf("table osm_roads does not exists in schema %s", dbschemaImport)
	}
}

func TestDeploy(t *testing.T) {
	ts.deployOsm(t)
	if ts.tableExists(t, dbschemaImport, "osm_roads") != false {
		t.Fatalf("table osm_roads exists in schema %s", dbschemaImport)
	}
	if ts.tableExists(t, dbschemaProduction, "osm_roads") != true {
		t.Fatalf("table osm_roads does not exists in schema %s", dbschemaProduction)
	}
}

type checkElem struct {
	table   string
	id      int64
	osmType string
	tags    map[string]string
}

func assertRecordsMissing(t *testing.T, elems []checkElem) {
	for _, e := range elems {
		if ts.queryExists(t, e.table, e.id) {
			t.Errorf("found %d in %d", e.id, e.table)
		}
	}
}

func assertRecords(t *testing.T, elems []checkElem) {
	for _, e := range elems {
		keys := make([]string, 0, len(e.tags))
		for k, _ := range e.tags {
			keys = append(keys, k)
		}
		r := ts.query(t, e.table, e.id, keys)
		if e.osmType == "" {
			if r.missing {
				continue
			}
			t.Errorf("got unexpected record %d", r.id)
		}
		if r.osmType != e.osmType {
			t.Errorf("got unexpected type %s != %s", r.osmType, e.osmType)
		}
		for k, v := range e.tags {
			if r.tags[k] != v {
				t.Errorf("%s does not match for %d %s != %s", k, e.id, r.tags[k], v)
			}
		}
	}
}

func assertValid(t *testing.T, e checkElem) {
	geom := ts.queryGeom(t, e.table, e.id)
	if !ts.g.IsValid(geom) {
		t.Fatalf("geometry of %d is invalid", e.id)
	}
}

func assertArea(t *testing.T, e checkElem, expect float64) {
	geom := ts.queryGeom(t, e.table, e.id)
	if !ts.g.IsValid(geom) {
		t.Fatalf("geometry of %d is invalid", e.id)
	}
	actual := geom.Area()
	if math.Abs(expect-actual) > 1 {
		t.Errorf("unexpected size of %d %f!=%f", e.id, actual, expect)
	}
}

func assertLength(t *testing.T, e checkElem, expect float64) {
	geom := ts.queryGeom(t, e.table, e.id)
	if !ts.g.IsValid(geom) {
		t.Fatalf("geometry of %d is invalid", e.id)
	}
	actual := geom.Length()
	if math.Abs(expect-actual) > 1 {
		t.Errorf("unexpected size of %d %f!=%f", e.id, actual, expect)
	}
}

func TestLandusageToWaterarea1(t *testing.T) {
	// Parks inserted into landusages
	// t.assert_cached_way(11001)
	// t.assert_cached_way(12001)
	// t.assert_cached_way(13001)

	assertRecords(t, []checkElem{
		{"osm_waterareas", 11001, "", nil},
		{"osm_waterareas", -12001, "", nil},
		{"osm_waterareas", -13001, "", nil},

		{"osm_waterareas_gen0", 11001, "", nil},
		{"osm_waterareas_gen0", -12001, "", nil},
		{"osm_waterareas_gen0", -13001, "", nil},

		{"osm_waterareas_gen1", 11001, "", nil},
		{"osm_waterareas_gen1", -12001, "", nil},
		{"osm_waterareas_gen1", -13001, "", nil},

		{"osm_landusages", 11001, "park", nil},
		{"osm_landusages", -12001, "park", nil},
		{"osm_landusages", -13001, "park", nil},

		{"osm_landusages_gen0", 11001, "park", nil},
		{"osm_landusages_gen0", -12001, "park", nil},
		{"osm_landusages_gen0", -13001, "park", nil},

		{"osm_landusages_gen1", 11001, "park", nil},
		{"osm_landusages_gen1", -12001, "park", nil},
		{"osm_landusages_gen1", -13001, "park", nil},
	})
}

func TestChangedHoleTags1(t *testing.T) {
	// Multipolygon relation with untagged hole
	// t.assert_cached_way(14001)
	// t.assert_cached_way(14011)

	assertRecords(t, []checkElem{
		{"osm_waterareas", 14011, "", nil},
		{"osm_waterareas", -14011, "", nil},
		{"osm_landusages", -14001, "park", nil},
	})
}

func TestSplitOuterMultipolygonWay1(t *testing.T) {
	// Single outer way of multipolygon was inserted.
	assertRecords(t, []checkElem{
		{"osm_roads", 15002, "", nil},
		{"osm_landusages", -15001, "park", nil},
	})
	assertArea(t, checkElem{"osm_landusages", -15001, "park", nil}, 9816216452)
}

func TestMergeOuterMultipolygonWay1(t *testing.T) {
	// Splitted outer way of multipolygon was inserted.
	assertRecords(t, []checkElem{
		{"osm_landusages", -16001, "park", nil},
		{"osm_roads", 16002, "residential", nil},
	})
	assertArea(t, checkElem{"osm_landusages", -16001, "park", nil}, 12779350582)
}

func TestBrokenMultipolygonWays(t *testing.T) {
	// MultiPolygons with broken outer ways are handled.
	// outer way does not merge (17002 has one node)

	assertRecords(t, []checkElem{
		{"osm_landusages", -17001, "", nil},
		{"osm_roads", 17001, "residential", nil},
		{"osm_roads", 17002, "", nil},
	})

	// outer way does not merge (17102 has no nodes)
	assertRecords(t, []checkElem{
		{"osm_landusages", -17101, "", nil},
		{"osm_roads", 17101, "residential", nil},
		{"osm_roads", 17102, "", nil},
	})
}

func TestNodeWayInsertedTwice(t *testing.T) {
	// Way with multiple mappings is inserted twice in same table
	//     rows = t.query_row(t.db_conf, 'osm_roads', 18001)
	//     rows.sort(key=lambda x: x['type'])
	rows := ts.queryRows(t, "osm_roads", 18001)
	if len(rows) != 2 || rows[0].osmType != "residential" || rows[1].osmType != "tram" {
		t.Errorf("unexpected roads: %v", rows)
	}
}

func TestOuterWayNotInserted(t *testing.T) {
	// Outer way with different tag is not inserted twice into same table

	assertRecords(t, []checkElem{
		{"osm_landusages", -19001, "farmland", nil},
		{"osm_landusages", 19002, "farmyard", nil},
		{"osm_landusages", 19001, "", nil},
	})
}

func TestOuterWayInserted(t *testing.T) {
	// Outer way with different tag is inserted twice into different table

	assertRecords(t, []checkElem{
		{"osm_landusages", 19101, "farm", nil},
		{"osm_landusages", 19102, "farmyard", nil},
		{"osm_admin", -19101, "administrative", nil},
	})
}

func TestNodeWayRefAfterDelete1(t *testing.T) {
	// Nodes refereces way
	//     data = t.cache_query(nodes=[20001, 20002], deps=True)
	//     assert '20001' in data['nodes']['20001']['ways']
	//     assert '20001' in data['nodes']['20002']['ways']
	assertRecords(t, []checkElem{
		{"osm_roads", 20001, "residential", nil},
		{"osm_barrierpoints", 20001, "block", nil},
	})
}

func TestWayRelRefAfterDelete1(t *testing.T) {
	// Ways references relation
	//     data = t.cache_query(ways=[21001], deps=True)
	//     assert data['ways']['21001']['relations'].keys() == ['21001']
	assertRecords(t, []checkElem{
		{"osm_roads", 21001, "residential", nil},
		{"osm_landusages", -21001, "park", nil},
	})
}

func TestRelationWayNotInserted(t *testing.T) {
	// Part of relation was inserted only once.

	assertRecords(t, []checkElem{
		{"osm_landusages", -9001, "park", map[string]string{"name": "rel 9001"}},
		{"osm_landusages", 9009, "", nil},
		{"osm_landusages", -9101, "park", map[string]string{"name": "rel 9101"}},
		{"osm_landusages", 9109, "", nil},
		{"osm_landusages", 9110, "scrub", nil},
	})
}

func TestRelationWaysInserted(t *testing.T) {
	// Outer ways of multipolygon are inserted.

	assertRecords(t, []checkElem{
		{"osm_landusages", -9201, "park", map[string]string{"name": "9209"}},
		{"osm_landusages", 9201, "", nil},
		// outer ways of multipolygon stand for their own
		{"osm_roads", 9209, "secondary", map[string]string{"name": "9209"}},
		{"osm_roads", 9210, "residential", map[string]string{"name": "9210"}},

		// no name on relation
		{"osm_landusages", -9301, "park", map[string]string{"name": ""}},
		// outer ways of multipolygon stand for their own
		{"osm_roads", 9309, "secondary", map[string]string{"name": "9309"}},
		{"osm_roads", 9310, "residential", map[string]string{"name": "9310"}},
	})

}

func TestRelationWayInserted(t *testing.T) {
	// Part of relation was inserted twice.

	assertRecords(t, []checkElem{
		{"osm_landusages", -8001, "park", map[string]string{"name": "rel 8001"}},
		{"osm_roads", 8009, "residential", nil},
	})
}

func TestSingleNodeWaysNotInserted(t *testing.T) {
	// Ways with single/duplicate nodes are not inserted.

	assertRecords(t, []checkElem{
		{"osm_landusages", 30001, "", nil},
		{"osm_landusages", 30002, "", nil},
		{"osm_landusages", 30003, "", nil},
	})
}

func TestPolygonWithDuplicateNodesIsValid(t *testing.T) {
	// Polygon with duplicate nodes is valid.

	assertValid(t, checkElem{"osm_landusages", 30005, "park", nil})
}

func TestIncompletePolygons(t *testing.T) {
	// Non-closed/incomplete polygons are not inserted.

	assertRecords(t, []checkElem{
		{"osm_landusages", 30004, "", nil},
		{"osm_landusages", 30006, "", nil},
	})
}

func TestResidentialToSecondary(t *testing.T) {
	// Residential road is not in roads_gen0/1.

	assertRecords(t, []checkElem{
		{"osm_roads", 40001, "residential", nil},
		{"osm_roads_gen0", 40001, "", nil},
		{"osm_roads_gen1", 40002, "", nil},
	})
}

func TestRelationBeforeRemove(t *testing.T) {
	// Relation and way is inserted.

	assertRecords(t, []checkElem{
		{"osm_buildings", 50011, "yes", nil},
		{"osm_landusages", -50021, "park", nil},
	})
}

func TestRelationWithoutTags(t *testing.T) {
	// Relation without tags is inserted.

	assertRecords(t, []checkElem{
		{"osm_buildings", 50111, "", nil},
		{"osm_buildings", -50121, "yes", nil},
	})
}

func TestDuplicateIds(t *testing.T) {
	// Relation/way with same ID is inserted.

	assertRecords(t, []checkElem{
		{"osm_buildings", 51001, "way", nil},
		{"osm_buildings", -51001, "mp", nil},
		{"osm_buildings", 51011, "way", nil},
		{"osm_buildings", -51011, "mp", nil},
	})
}

func TestGeneralizedBananaPolygonIsValid(t *testing.T) {
	// Generalized polygons are valid.

	assertValid(t, checkElem{"osm_landusages", 7101, "", nil})
	// simplified geometies are valid too
	assertValid(t, checkElem{"osm_landusages_gen0", 7101, "", nil})
	assertValid(t, checkElem{"osm_landusages_gen1", 7101, "", nil})
}

func TestGeneralizedLinestringIsValid(t *testing.T) {
	// Generalized linestring is valid.

	// geometry is not simple, but valid
	assertLength(t, checkElem{"osm_roads", 7201, "primary", nil}, 1243660.044819)
	if ts.g.IsSimple(ts.queryGeom(t, "osm_roads", 7201)) {
		t.Errorf("expected non-simple geometry for 7201")
	}
	// check that geometry 'survives' simplification
	assertLength(t, checkElem{"osm_roads_gen0", 7201, "primary", nil}, 1243660.044819)
	assertLength(t, checkElem{"osm_roads_gen1", 7201, "primary", nil}, 1243660.044819)
}

func TestRingWithGap(t *testing.T) {
	// Multipolygon and way with gap (overlapping but different endpoints) gets closed
	assertValid(t, checkElem{"osm_landusages", -7301, "", nil})
	assertValid(t, checkElem{"osm_landusages", 7311, "", nil})
}

func TestMultipolygonWithOpenRing(t *testing.T) {
	// Multipolygon is inserted even if there is an open ring/member
	assertValid(t, checkElem{"osm_landusages", -7401, "", nil})
}

func TestUpdatedNodes1(t *testing.T) {
	// Zig-Zag line is inserted.
	assertLength(t, checkElem{"osm_roads", 60000, "", nil}, 14035.61150207768)
}

func TestUpdateNodeToCoord1(t *testing.T) {
	// Node is inserted with tag.
	//     coords = t.cache_query(nodes=(70001, 70002))
	//     assert coords['nodes']["70001"]["tags"] == {"amenity": "police"}
	//     assert "tags" not in coords['nodes']["70002"]
	assertRecords(t, []checkElem{
		{"osm_amenities", 70001, "police", nil},
		{"osm_amenities", 70002, "", nil},
	})
}

func TestEnumerateKey(t *testing.T) {
	// Enumerate from key.
	assertRecords(t, []checkElem{
		{"osm_landusages", 100001, "park", map[string]string{"enum": "1"}},
		{"osm_landusages", 100002, "park", map[string]string{"enum": "0"}},
		{"osm_landusages", 100003, "wood", map[string]string{"enum": "15"}},
	})
}
