# Domain Docs

本仓库采用 single-context：根级 `GLOSSARY.md` 与 `docs/adr/`。

## 探索代码前

- 读取根级 `GLOSSARY.md`。
- 读取 `docs/adr/` 中与当前工作相关的 ADR。

文件或目录不存在时，静默继续；不要仅因缺失而建议创建。
`/domain-modeling` 在术语或决策实际确定后按需创建领域文档。
`/grill-with-docs` 和 `/improve-codebase-architecture` 可调用该技能。

## 使用词汇表术语

在工单标题、重构建议、假设和测试名称中，使用 `GLOSSARY.md` 定义的领域术语。
避免使用词汇表明确排除的同义词。

需要的概念尚未收录时，先检查是否使用了项目不存在的概念。
确属词汇表缺口时，记录给 `/domain-modeling`。

## 标明 ADR 冲突

输出与现有 ADR 冲突时，明确指出对应 ADR 与重新讨论的理由。
