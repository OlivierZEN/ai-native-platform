# L2 Loop Budget — PostgreSQL、租户与元数据内核

- Status: completed; independent checker round 3 `PASS`; budget is closed until a new user-authorized Loop starts.

- Completion bound: TASK-010、TASK-012、TASK-011、TASK-013 全部完成并独立验证；没有未经授权的时间硬停止。
- Max active work item: 1。
- Max failed attempts per item: 3。
- Max direct dependency additions: 3；每项必须有版本、许可、checksum 和传递依赖证据。
- Checkpoint size: 每个实现检查点原则上不超过 10 个源/迁移文件；跨层垂直切片可在日志中说明后扩至 15 个，但不得扩大功能范围。
- Database environments: 1 个专用临时 PostgreSQL 16 容器；远程/生产数据库预算为 0。
- Verifier agents: 1；checker 不修改实现。
- Remote write/publish/merge/deploy budget: 0。
- Reserve: 最终至少保留一次完整 maker 验证和一次独立 checker 复核。
- Kill criteria: denylist 命中、凭据/真实数据暴露、许可不明、连续三次失败或无法建立可信的租户隔离证据。
