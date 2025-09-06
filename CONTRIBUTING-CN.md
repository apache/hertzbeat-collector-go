# 贡献者指南 

> 我们致力于维护一个互帮互助、快乐的社区，欢迎每一位贡献者加入！

### 贡献方式

> 在 HertzBeat 社区，贡献方式有很多：

- 💻**代码**：可以帮助社区完成任务、开发新特性或修复 bug；
- ⚠️**测试**：参与单元测试、集成测试、e2e 测试的编写；
- 📖**文档**：完善文档，帮助用户更好地了解和使用 HertzBeat；
- 📝**博客**：撰写相关文章，帮助社区推广；
- 🤔**讨论**：参与新特性的讨论，将你的想法融入 HertzBeat；
- 💡**布道**：宣传推广 HertzBeat 社区，在 meetup 或 summit 中演讲；
- 💬**建议**：提出建议，促进社区健康发展；

更多贡献类型参见 [Contribution Types](https://allcontributors.org/docs/en/emoji-key)

即便是小到错别字的修正我们也非常欢迎 :)

### 让 HertzBeat 跑起来

> 让 HertzBeat 代码在你的开发环境中运行，并支持断点调试。
> 本项目前后端分离，需分别启动后端 [manager](manager) 和前端 [web-app](web-app)。

#### 后端启动

1. 需要 `maven3+`、`java17` 和 `lombok` 环境
2. （可选）修改配置文件：`manager/src/main/resources/application.yml`
3. 在项目根目录执行：`mvn clean install -DskipTests`
4. JVM 参数加入：`--add-opens=java.base/java.nio=org.apache.arrow.memory.core,ALL-UNNAMED`
5. 启动 `springboot manager` 服务：`manager/src/main/java/org/apache/hertzbeat/hertzbeat-manager/Manager.java`

#### 前端启动

1. 需要 `Node.js` 和 `yarn` 环境，Node.js >= 18
2. 进入 `web-app` 目录：`cd web-app`
3. 安装 yarn：`npm install -g yarn`
4. 安装依赖：`yarn install` 或 `yarn install --registry=https://registry.npmmirror.com`
5. 全局安装 angular-cli：`yarn global add @angular/cli@15` 或 `yarn global add @angular/cli@15 --registry=https://registry.npmmirror.com`
6. 后端启动后，在 web-app 目录下启动前端：`ng serve --open`
7. 浏览器访问 localhost:4200，默认账号/密码：_admin/hertzbeat_

### 寻找任务

在 GitHub issue 列表和邮件列表中查找感兴趣的任务，带有 good first issue 或 status: volunteer wanted 标签的 issue 欢迎新手参与。

如有新想法，也可在 GitHub Discussion 提出或联系社区。

### 提交 Pull Request

1. Fork 目标仓库 [hertzbeat repository](https://github.com/apache/hertzbeat)
2. 用 git 下载代码：

   ```shell
   git clone git@github.com:${YOUR_USERNAME}/hertzbeat.git # 推荐
   ```

3. 下载完成后，参考入门指南或 README 初始化项目
4. 切换新分支并开发：

   ```shell
   git checkout -b a-feature-branch # 推荐
   ```

5. 按规范提交 commit：

   ```shell
   git add <modified file/path>
   git commit -m '[docs]feature: necessary instructions' # 推荐
   ```

6. 推送到远程仓库：

   ```shell
   git push origin a-feature-branch
   ```

7. 在 GitHub 上发起新的 PR（Pull Request）

PR 标题需符合规范，并写明必要说明，便于代码审查。

### 等待 PR 合并

PR 提交后，Committer 或社区成员会进行代码审查（Code Review），并提出修改建议或讨论。请及时关注你的 PR。

如需修改，无需新建 PR，直接在原分支提交并推送即可自动更新。

项目有严格的 CI 检查流程，PR 提交后会自动触发 CI，请关注是否通过。

最终 Committer 会将 PR 合并到主分支。

### 代码合并后

合并后可删除本地和远程开发分支：

```shell
git branch -d a-dev-branch
git push origin --delete a-dev-branch
```

主分支同步上游仓库：

```shell
git remote add upstream https://github.com/apache/hertzbeat.git # 若已执行可跳过
git checkout master
git pull upstream master
```

### HertzBeat 改进提案（HIP）

如有重大新特性（如支持指标推送网关、日志监控等），需编写 HertzBeat 改进提案（HIP），流程见 [HertzBeat hip](https://github.com/apache/hertzbeat/tree/master/hip)。

### 如何成为 Committer？

重复上述流程，持续活跃贡献，你就有机会成为 Committer！

### 加入讨论交流

[加入邮件列表](https://lists.apache.org/list.html?dev@hertzbeat.apache.org)：发送邮件至 `dev-subscribe@hertzbeat.apache.org` 订阅。

添加微信号 `ahertzbeat` 入微信群。

## 🥐 架构

- **[manager](https://github.com/apache/hertzbeat/tree/master/hertzbeat-manager)** 提供监控管理、系统管理基础服务
- **[collector](https://github.com/apache/hertzbeat/tree/master/collector)** 提供指标数据采集服务
- **[warehouse](https://github.com/apache/hertzbeat/tree/master/warehouse)** 提供监控数据仓储服务
- **[alerter](https://github.com/apache/hertzbeat/tree/master/hertzbeat-alerter)** 提供告警服务
- **[web-app](https://github.com/apache/hertzbeat/tree/master/web-app)** 提供可视化控制台页面

> 详见各模块说明。

![hertzBeat](home/static/img/docs/hertzbeat-arch.png)
