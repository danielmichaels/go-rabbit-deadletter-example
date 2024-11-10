# Go Rabbit Example

> This is a very simple example of how to use rabbitmq with Go and have failed
messages sent to a dead letter queue.

## What is this

In this example the failed messages are sent to the x-dead-letter-exchange and forwarded
to a retry queue which has a TTL set. Once the TTL expires the message is sent to its 
original queue.

At time of writing, this does not handle messages that would indefinitely fail. But, this
can easily be achieved by reading the Message metadata headers for the retry count and acking
the message if the retry count is greater than the max number of retries.

The library used does not handle reconnections to the rabbit node. For that you will need to
write that functionality yourself, or use something like [wagslane/go-rabbitmq](https://github.com/wagslane/go-rabbitmq).

## How to run 

You will need [Task](https://taskfile.dev), or to read the commands from the Taskfile and excute them manually.

Run the task in this order:

- `task startrabbit`
- `task consume`
- Open another terminal
- `task publish -- 200` (publishes 200 messages to the queue)

Now go to <http://localhost:15672> using `guest:guest` to login. Look at the Queues and Streams
tab to view the messages being consumed and failed messages being routed to the `retry_*` queue.

After some time the messages in the retry queue will be forwarded back to the to `customers_created` queue
and re-attempted. Eventually all messages will succeed. 

### Acknowledgements

Much of this code is taken from Programming Percy's [go rabbit][gr] blog post.

[gr]: https://programmingpercy.tech/blog/event-driven-architecture-using-rabbitmq/