package move_fields

import (
	"reflect"
	"testing"

	"github.com/elastic/beats/v7/libbeat/beat"

	"github.com/elastic/elastic-agent-libs/mapstr"
)

func TestMoveFields(t *testing.T) {
	cases := []struct {
		in, except mapstr.M
		p          *moveFields
	}{
		{
			mapstr.M{"app": mapstr.M{"version": 1, "method": "2"}, "other": 3},
			mapstr.M{"app": mapstr.M{"method": "2"}, "rpc": mapstr.M{"version": 1}, "other": 3},
			&moveFields{config: moveFieldsConfig{
				ParentPath: "app",
				From:       nil,
				To:         "rpc.",
				Exclude:    []string{"method"},
				excludeMap: map[string]bool{"method": true},
			}},
		},
		{
			mapstr.M{"app": mapstr.M{"version": 1, "method": "2"}, "other": 3},
			mapstr.M{"app": mapstr.M{}, "rpc": mapstr.M{"method": "2", "version": 1}, "other": 3},
			&moveFields{config: moveFieldsConfig{
				ParentPath: "app",
				From:       nil,
				To:         "rpc.",
				Exclude:    nil,
				excludeMap: nil,
			}},
		},
		{
			mapstr.M{"app": mapstr.M{"version": 1, "method": "2"}, "other": 3},
			mapstr.M{"app": mapstr.M{}, "rpc_method": "2", "rpc_version": 1, "other": 3},
			&moveFields{config: moveFieldsConfig{
				ParentPath: "app",
				From:       nil,
				To:         "rpc_",
				Exclude:    nil,
				excludeMap: nil,
			}},
		},
		{
			mapstr.M{"app.version": 1, "other": 3},
			mapstr.M{"app": mapstr.M{"version": 1}, "other": 3},
			&moveFields{config: moveFieldsConfig{
				ParentPath: "",
				From:       []string{"app.version"},
				To:         "",
				Exclude:    nil,
				excludeMap: nil,
			}},
		},
		{
			mapstr.M{"app": mapstr.M{"version": 1, "method": "2"}, "other": 3},
			mapstr.M{"my_prefix_app": mapstr.M{"version": 1, "method": "2"}, "my_prefix_other": 3},
			&moveFields{config: moveFieldsConfig{
				ParentPath: "",
				From:       nil,
				To:         "my_prefix_",
				Exclude:    nil,
				excludeMap: nil,
			}},
		},
	}

	for idx, c := range cases {
		evt := &beat.Event{Fields: c.in.Clone()}
		out, err := c.p.Run(evt)
		if err != nil {
			t.Fatal(err)
		}
		except, output := c.except.String(), out.Fields.String()
		if !reflect.DeepEqual(c.except, out.Fields) {
			t.Fatalf("move field test case failed, out: %s, except: %s, index: %d\n",
				output, except, idx)
		}
	}
}
