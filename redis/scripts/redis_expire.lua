if (redis.call('hexists', KEYS[1], ARGV[1]) == 1)
then
    redis.call('pexpire', KEYS[1], ARGV[2]);
    return 1;
end;
return nil