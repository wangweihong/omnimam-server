package code

// gitlab: spec-v1.22.0 server, project, pipeline, and access errors.
const (
	// @HTTP 200
	// @CN GitLabServer 名称已存在。
	// @EN The GitLabServer name already exists.
	ErrGitLabServerNameConflict int = 250200
	// @HTTP 200
	// @CN GitLabServer 不存在或当前不可见。
	// @EN The GitLabServer does not exist or is not visible.
	ErrGitLabServerNotFound int = 250201
	// @HTTP 200
	// @CN GitLabServer 连接、credential 或 Namespace 检测失败。
	// @EN GitLabServer connectivity, credential, or Namespace validation failed.
	ErrGitLabServerConnectionFailed int = 250202
	// @HTTP 200
	// @CN GitLabServer 仍有关联 GitLabProject，不能删除。
	// @EN The GitLabServer still has GitLabProject projections and cannot be deleted.
	ErrGitLabServerHasProjects int = 250203
	// @HTTP 200
	// @CN GitLabProject 不存在或当前不可见。
	// @EN The GitLabProject does not exist or is not visible.
	ErrGitLabProjectNotFound int = 250400
	// @HTTP 200
	// @CN GitLabServer 尚未通过连接检测。
	// @EN The GitLabServer has not passed the connectivity test.
	ErrGitLabProjectServerNotReady int = 250401
	// @HTTP 200
	// @CN GitLab 远端 Project 操作失败。
	// @EN The remote GitLab Project operation failed.
	ErrGitLabProjectRemoteFailed int = 250402
	// @HTTP 200
	// @CN GitLabProject 本地投影写入失败。
	// @EN The local GitLabProject projection could not be persisted.
	ErrGitLabProjectProjectionFailed int = 250403
	// @HTTP 200
	// @CN GitLab Pipeline 任务输入无效。
	// @EN The GitLab Pipeline task input is invalid.
	ErrGitLabPipelineInvalid int = 250600
	// @HTTP 200
	// @CN GitLab Pipeline 创建失败。
	// @EN The GitLab Pipeline could not be created.
	ErrGitLabPipelineCreateFailed int = 250601
	// @HTTP 200
	// @CN GitLab Pipeline 执行失败。
	// @EN The GitLab Pipeline failed.
	ErrGitLabPipelineFailed int = 250602
	// @HTTP 200
	// @CN GitLab Pipeline 已取消。
	// @EN The GitLab Pipeline was canceled.
	ErrGitLabPipelineCanceled int = 250603
	// @HTTP 200
	// @CN 当前主体无权访问 GitLab 资源。
	// @EN The current principal is not authorized to access the GitLab resource.
	ErrGitLabAccessDenied int = 250800
)
