if (redis.call('exists', KEYS[1]) == 0)
then
    redis.call('publish', KEYS[2], ARGV[2]);
    return 1;
end;

if (redis.call('hexists', KEYS[1], ARGV[1]) == 0)
then
    return 0;
end;

local counter = redis.call('hincrby', KEYS[1], ARGV[1], -1);

if (counter <= 0)
then
    redis.call('del', KEYS[1]);
    redis.call('publish', KEYS[2], ARGV[2]);
    return 1;
end;
return nil;