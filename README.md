```bash
git config --global core.fileMode false
go env -w GOPROXY=https://goproxy.cn,direct
go install github.com/air-verse/air@latest
```

# Tests for RAG

## 原人.md

```md
# 原人

原人是由哈基米自研的一款开放世界冒险 RPG。你将在游戏中探索一个被称作瓦特乐的幻想世界。在这广阔的世界中，你可以踏遍七国，邂逅性格各异、能力独特的同伴，与他们一同对抗强敌，踏上寻回血亲之路；也可以不带目的地漫游，沉浸在充满生机的世界里，让好奇心驱使自己发掘各个角落的奥秘……直到你与分离的血亲重聚，在终点见证一切事物的沉淀
```

## 哈基米.md

```md
# 哈基米（也称为马哈鱼）

哈基米，也称为马哈鱼，成立于 2020 年，致力于为用户提供美好的、超出预期的产品与内容。哈基米多年来秉持技术自主创新，坚持走原创精品之路，围绕原创 IP 打造了涵盖漫画、动画、游戏、音乐、小说及动漫周边的全产业链
```

## Go mod

Q: 我在 github 的 github.com/jane_doe/repo 下有一个 go work 项目 (类似前端 monorepo), 分为 5 个子包: repo1、repo2、repo3、repo4、repo5, 其中 repo1、repo2、repo3、repo4 互不依赖, repo5 依赖 repo1、repo2、repo3 和 repo4; 现在我希望将 repo1、repo2、repo3 和 repo4 发布为 go mod, 我应该怎么做

A

```bash
git tag repo1/v0.0.1
git push origin repo1/v0.0.1

git tag repo2/v0.0.1
git push origin repo2/v0.0.1

git tag repo3/v0.0.1
git push origin repo3/v0.0.1

git tag repo4/v0.0.1
git push origin repo4/v0.0.1
```

## Install

```bash
go mod download github.com/hangtiancheng/swifty.go/swifty_cache@v0.0.1
go mod download github.com/hangtiancheng/swifty.go/swifty_http@v0.0.1
go mod download github.com/hangtiancheng/swifty.go/swifty_orm@v0.0.1
go mod download github.com/hangtiancheng/swifty.go/swifty_rpc@v0.0.1
```

/skill-creator

- 阅读 swifty_cache 源代码, 创建 '.github/skills/swifty_cache/SKILL.md' Agent Skill
- 阅读 swifty_http 源代码，创建 '.github/skills/swifty_http/SKILL.md' Agent Skill
- 阅读 swifty_orm 源代码，创建 '.github/skills/swifty_orm/SKILL.md' Agent Skill
- 阅读 swifty_rpc 源代码，创建 '.github/skills/swifty_rpc/SKILL.md' Agent Skill

使用专业的英语, 保证专业、全面、Agent 友好
