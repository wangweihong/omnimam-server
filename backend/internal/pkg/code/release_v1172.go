package code

const (
	// @HTTP 200
	// @CN Endpoint 或所属 Runtime 未达到可解析状态，或 Endpoint 已过期、撤销。
	// @EN The endpoint or its runtime is not ready for resolution, or the endpoint has expired or been revoked.
	ErrInfraEndpointNotReady int = 240804

	// @HTTP 200
	// @CN 声明输出缺失、不是普通文件、路径逃逸或实际字节收集失败。
	// @EN The declared output is missing, is not a regular file, escapes the output root, or could not be collected.
	ErrInfraOutputCollectionFailed int = 240805

	// @HTTP 200
	// @CN RuntimeOutput 内容尚未收集、已清理或当前 Task Worker 无法读取。
	// @EN The RuntimeOutput content has not been collected, has been cleaned up, or is unavailable to the current Task Worker.
	ErrInfraOutputContentUnavailable int = 240806

	// @HTTP 200
	// @CN RuntimeOutput、传输字节与 Artifact 的大小或 SHA-256 不一致。
	// @EN The size or SHA-256 differs across the RuntimeOutput, transferred bytes, and Artifact.
	ErrInfraOutputIntegrityMismatch int = 240807
)
