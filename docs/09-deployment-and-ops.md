# 09 Deployment And Ops

## 目标

定义本地开发、测试、部署、日志、监控和告警要求，作为工程化落地依据。

## V1 部署方式

建议使用 Docker Compose。

最小服务：

- frontend
- backend
- worker
- postgres
- redis

## 环境变量建议

- `POSTGRES_DSN`
- `REDIS_ADDR`
- `ARXIV_USER_AGENT`
- `LLM_API_KEY`
- `WECHAT_APP_ID`
- `WECHAT_APP_SECRET`

## 日志要求

- 请求日志
- 任务日志
- 外部接口日志
- 错误日志

## 指标建议

- 每日抓取数
- 推荐成功率
- 解析成功率
- 草稿生成成功率
- 发布成功率

## 告警建议

- 定时任务未执行
- 当日候选为 0
- 发布失败
- 连续多次 LLM 调用失败

## 与代码生成的关系

本文件会直接影响：

- `docker-compose.yml`
- `.env.example`
- 日志中间件
- 健康检查接口

## 需要你补充的信息

- 部署环境是本机、云主机还是内网服务器
- 是否需要 HTTPS 反向代理
- 是否要接企业微信/飞书告警
- 是否需要分 dev/test/prod 三套环境

