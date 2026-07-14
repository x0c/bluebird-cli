package cmdtree

import (
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/version"
)

func (a *App) astroCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "astro",
		Short: "天文日历（月相、日出日落、流星雨等，纯本地计算）",
	}
	cmd.AddCommand(a.astroCalendarCmd(opts))
	return cmd
}

func (a *App) astroCalendarCmd(opts *globalOptions) *cobra.Command {
	pf := &placeFlags{}
	var days int
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "查询天文日历（必须指定地点）",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)

			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}

			in := placeInputFrom(cmd, pf)
			result, cmdErr := a.resolvePlace(cmd.Context(), client, in, opts.NoInput)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}
			if result == nil {
				return a.emitErr(opts, jsonMode, start,
					output.NewError(output.CodeUsageError, "astro calendar 必须指定地点").
						WithHint("加 --place <地名> 或 --lat/--lon"))
			}

			query := url.Values{}
			query.Set("lat", strconv.FormatFloat(result.Latitude, 'f', -1, 64))
			query.Set("lon", strconv.FormatFloat(result.Longitude, 'f', -1, 64))
			if days > 0 {
				query.Set("days", strconv.Itoa(days))
			}

			var data map[string]any
			if err := client.Get(cmd.Context(), "/api/astro/calendar", query, &data); err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}

			var resolvedPlace any
			if result.Resolved != nil {
				resolvedPlace = result.Resolved
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start,
				opts.renderOptions(jsonMode, []string{"days"}),
				output.Result{Data: data, ResolvedPlace: resolvedPlace}, renderAstroHuman)
			return exitError(exit)
		},
	}
	addPlaceFlags(cmd, pf)
	cmd.Flags().IntVar(&days, "days", 15, "天数（1-30）")
	return cmd
}

func renderAstroHuman(w io.Writer, data any, _ bool) error {
	m, ok := data.(map[string]any)
	if !ok {
		_, err := fmt.Fprintln(w, data)
		return err
	}
	daysRaw, ok := m["days"].([]any)
	if !ok || len(daysRaw) == 0 {
		_, err := fmt.Fprintln(w, "暂无天文日历数据。")
		return err
	}
	for _, d := range daysRaw {
		dm, _ := d.(map[string]any)
		date, _ := dm["date"].(string)
		phase, _ := dm["moonPhase"].(string)
		if _, err := fmt.Fprintf(w, "%s 月相：%s\n", date, phase); err != nil {
			return err
		}
		events, _ := dm["events"].([]any)
		for _, e := range events {
			em, _ := e.(map[string]any)
			name, _ := em["name"].(string)
			desc, _ := em["description"].(string)
			if _, err := fmt.Fprintf(w, "  - %s：%s\n", name, desc); err != nil {
				return err
			}
		}
	}
	return nil
}
