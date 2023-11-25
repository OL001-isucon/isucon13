DROP TRIGGER IF EXISTS update_user_stats;

DELIMITER //
CREATE TRIGGER update_user_stats
AFTER INSERT ON reactions
FOR EACH ROW
BEGIN
    -- Update reaction_count in user_stats table
    UPDATE user_stats
    SET reaction_count = reaction_count + 1
    WHERE user_id = NEW.user_id;

    -- If no record exists for the user_id, insert a new one
    IF ROW_COUNT() = 0 THEN
        INSERT INTO user_stats (user_id, reaction_count, tip_amount)
        VALUES (NEW.user_id, 1, 0);
    END IF;
END;
//
DELIMITER ;

DROP TRIGGER IF EXISTS update_user_stats_livecomments;

DELIMITER //
CREATE TRIGGER update_user_stats_livecomments
AFTER INSERT ON livecomments
FOR EACH ROW
BEGIN
    -- Update tip_amount in user_stats table
    UPDATE user_stats
    SET tip_amount = tip_amount + NEW.tip
    WHERE user_id = NEW.user_id;

    -- If no record exists for the user_id, insert a new one
    IF ROW_COUNT() = 0 THEN
        INSERT INTO user_stats (user_id, reaction_count, tip_amount)
        VALUES (NEW.user_id, 0, NEW.tip);
    END IF;
END;
//
DELIMITER ;
