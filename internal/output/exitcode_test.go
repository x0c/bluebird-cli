package output

import "testing"

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		code string
		want int
	}{
		{"", ExitOK},
		{CodeUsageError, ExitUsage},
		{CodeInvalidScene, ExitUsage},
		{CodeBadRequest, ExitUsage},
		{CodeNotFound, ExitNotFound},
		{CodePlaceNotFound, ExitNotFound},
		{CodeNoLocation, ExitNotFound},
		{CodeUnauthorized, ExitAuth},
		{CodeAuthNotConfigured, ExitAuth},
		{CodePlaceAmbiguous, ExitConflict},
		{CodeWatchConflict, ExitConflict},
		{CodeServerError, ExitGeneral},
		{CodeNetworkError, ExitGeneral},
		{CodeRateLimited, ExitGeneral},
		{CodeFeatureUnavailable, ExitGeneral},
		{CodeTimeout, ExitTimeout},
		{"unknown_future_code", ExitGeneral},
	}
	for _, c := range cases {
		if got := ExitCodeFor(c.code); got != c.want {
			t.Errorf("ExitCodeFor(%q) = %d, want %d", c.code, got, c.want)
		}
	}
}
