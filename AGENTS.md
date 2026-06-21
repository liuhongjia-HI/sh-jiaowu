## Communication

- 默认使用中文回复用户，除非用户明确要求使用其他语言。
- 技术说明、评审结论、实施进度、最终回复均优先使用中文。
- 代码、命令、文件名、接口字段名保持原文，不强行翻译。

## Project Notes

- 后端目录为 `learning-api/`，不要改回 `data-api/`。
- 后端接口保持 `/api` 前缀，响应结构为 `{ code, message, data }`。
- 管理端操作人通过 `X-Operator-ID`、`X-Operator-Name` 透传。
- 学生端是原生微信小程序，目录为 `miniprogram/`。

