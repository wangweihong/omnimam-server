package code

// gitlab/appstudio: spec-v1.23.3 initialization diagnostics and recovery errors.
const (
	// @HTTP 200
	// @CN 没有 READY 且标记为 AppStudio 默认的 GitLabServer，请先完成代码仓库配置和连接检测。
	// @EN No READY GitLabServer is configured as the AppStudio default. Configure and test a repository connection first.
	ErrGitLabAppStudioDefaultServerUnavailable int = 250204
)
