# workerpool
支持排序的工作队列

# 功能列表
- FIFO任务队列。
- 自定义任务数据结构。
- 可以自定义权重进行任务排序。
- 同步、异步取消正在执行的任务。
- 队列中任务是否存在查询（包括正在进行的任务）。

# 需要任务自己实现的逻辑
- 任务执行结果、执行状态的保存入库。
- 任务执行进度保存与查询。
- 