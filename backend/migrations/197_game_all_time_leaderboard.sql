-- Support selecting each user's all-time best game score without scanning the
-- daily score history table in full.
CREATE INDEX IF NOT EXISTS idx_game_daily_scores_all_time_leaderboard
    ON game_daily_scores (game_id, user_id, score DESC, achieved_at ASC, local_date ASC);
