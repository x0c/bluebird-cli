package cmdtree

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/x0c/bluebird-cli/internal/api"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/version"
)

func (a *App) placeCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "place",
		Short: "地名搜索",
	}
	cmd.AddCommand(a.placeSearchCmd(opts))
	return cmd
}

func (a *App) placeSearchCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "按关键词搜索地点，返回候选坐标列表（多结果是正常输出，不是歧义错误）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}
			results, err := client.SearchPlaces(cmd.Context(), args[0])
			if err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}
			data := map[string]any{"results": results, "total": len(results)}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{Data: data}, renderPlaceSearchHuman)
			return exitError(exit)
		},
	}
}

func renderPlaceSearchHuman(w io.Writer, data any, _ bool) error {
	m, ok := data.(map[string]any)
	if !ok {
		_, err := fmt.Fprintln(w, data)
		return err
	}
	switch results := m["results"].(type) {
	case []api.PlaceResult:
		if len(results) == 0 {
			_, err := fmt.Fprintln(w, "没有找到匹配的地点。")
			return err
		}
		for i, r := range results {
			if _, err := fmt.Fprintf(w, "  [%d] %s (%.4f, %.4f) %s\n", i+1, r.DisplayName, r.Latitude, r.Longitude, r.Country); err != nil {
				return err
			}
		}
		return nil
	case []any:
		if len(results) == 0 {
			_, err := fmt.Fprintln(w, "没有找到匹配的地点。")
			return err
		}
		for i, ri := range results {
			rm, _ := ri.(map[string]any)
			name, _ := rm["displayName"].(string)
			lat, _ := rm["latitude"].(float64)
			lon, _ := rm["longitude"].(float64)
			country, _ := rm["country"].(string)
			if _, err := fmt.Fprintf(w, "  [%d] %s (%.4f, %.4f) %s\n", i+1, name, lat, lon, country); err != nil {
				return err
			}
		}
		return nil
	default:
		_, err := fmt.Fprintln(w, "没有找到匹配的地点。")
		return err
	}
}
