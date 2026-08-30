package app

import "testing"

// TestPreviewSummary 命令层透传 settings 到 core.PreviewSummaryLine。
func TestPreviewSummary(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(*App)
		total   int
		format  string
		invalid int
		want    string
	}{
		{
			name: "vcf_with_prefix_and_invalid",
			setup: func(a *App) {
				a.settings.PhoneValidate = true
				a.settings.VCFNamePrefix = "客户-"
				a.settings.VCFTimestamp = false
			},
			total: 8, format: "vcf", invalid: 2,
			want: "共 8 行，vcf 姓名前缀：客户-，其中 2 个手机号格式异常",
		},
		{
			name: "xlsx_passthrough",
			setup: func(a *App) {
				a.settings.PhoneValidate = true
				a.settings.VCFNamePrefix = "ignored"
			},
			total: 12, format: "xlsx", invalid: 0,
			want: "共 12 行",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t)
			if tc.setup != nil {
				tc.setup(a)
			}
			got := a.PreviewSummary(tc.total, tc.format, tc.invalid)
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}
