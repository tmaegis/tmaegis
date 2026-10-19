# 合规报告

- **主机名:** web-01
- **操作系统:** linux (amd64)
- **目标:** linux
- **规则目录:** `rules`
- **涉及的配置级别:** L1
- **扫描时间:** 2026-03-04 05:06 UTC
- **其他选项:** `--level 1`
- **版本:** TMAegis 1.2.3

## 摘要

**合规得分:** 25.0% - 存在风险

在 4 个已评估的控制项中，有 3 个未通过（其中 2 个为严重或高危）。

| 严重程度 | 通过 | 未通过 | 已跳过 | 人工审查 | 规则总数 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 严重 | 0 | 1 | 0 | 0 | 1 |
| 高 | 0 | 1 | 0 | 0 | 1 |
| 中 | 0 | 0 | 1 | 1 | 2 |
| 低 | 1 | 1 | 0 | 0 | 2 |
| **规则总数** | **1** | **3** | **1** | **1** | **6** |

## 按基准

| 基准 | 未通过 | 通过 | 得分 |
| --- | ---: | ---: | ---: |
| LINUX | 2 | 1 | 33.3% |
| DOCKER | 1 | 0 | 0.0% |

## 按严重程度统计的未通过控制项

### 严重 (1)

#### TMA-LINUX-1.1 - Shadow file permissions

- **状态:** 未通过
- **严重程度:** 严重
- **目标:** linux
- **参考:** CIS Linux Benchmark - 1.1

**扫描输出**

```
mode 644
```

**修复方法**

```
chmod 600 /etc/shadow
```

### 高 (1)

#### TMA-LINUX-1.2 - SSH root login

- **状态:** 未通过
- **严重程度:** 高
- **目标:** linux
- **参考:** CIS Linux Benchmark - 1.1

**扫描输出**

```
PermitRootLogin yes
```

**修复方法 (仅供指导)**

```
Set PermitRootLogin no
```

### 低 (1)

#### TMA-DOCKER-2.1 - Docker socket

- **状态:** 错误
- **严重程度:** 低
- **目标:** linux
- **参考:** CIS Linux Benchmark - 1.1

**修复方法:** 此控制项没有可用的修复说明。

## 人工审查

1 个控制项需要人工审查

### TMA-LINUX-1.4 - Review sudoers

- **严重程度:** 中
- **目标:** linux
- **参考:** CIS Linux Benchmark - 1.1

**验证方法**

```
Open /etc/sudoers and review it
```

## 不适用于此主机

1 个控制项不适用

- TMA-LINUX-1.5 - No GRUB here

---

由 TMAegis 生成。CIS Benchmarks 是 Center for Internet Security 的商标；TMAegis 与 CIS 无任何关联。

TMAegis(TM) 及 TMAegis 标志是项目创始人的商标。 TMAegis 是开源软件，采用 Mozilla Public License 2.0（MPL-2.0）许可。
