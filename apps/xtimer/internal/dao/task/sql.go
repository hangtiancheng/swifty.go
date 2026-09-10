package task

// SQLGetMinuteTaskCnt counts tasks grouped per minute between two timestamps.
const SQLGetMinuteTaskCnt = `SELECT LEFT(run_timer,16) AS minute,count(*) AS cnt FROM task WHERE run_timer >= '%s' AND run_timer < '%s' GROUP BY LEFT(run_timer,16) `
