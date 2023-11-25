DELIMITER //
CREATE TRIGGER update_user_stats
AFTER INSERT ON reactions
FOR EACH ROW
BEGIN
  INSERT INTO user_stats (user_id, reaction_count, tip_amount)
  VALUES (NEW.user_id, 1, 0) ON DUPLICATE KEY UPDATE reaction_count = reaction_count + 1;
END;
//
DELIMITER ;

DELIMITER //
CREATE TRIGGER update_user_stats_livecomments
AFTER INSERT ON livecomments
FOR EACH ROW
BEGIN
  INSERT INTO user_stats (user_id, reaction_count, tip_amount)
  VALUES (NEW.user_id, 0, NEW.tip) ON DUPLICATE KEY UPDATE tip_amount = tip_amount + NEW.tip;
END;
//
DELIMITER ;

DELIMITER //
CREATE TRIGGER update_user_insert
AFTER INSERT ON users
FOR EACH ROW
BEGIN
  INSERT INTO user_stats (user_id, reaction_count, tip_amount)
  VALUES (NEW.id, 0, 0);
END;
//
DELIMITER ;
