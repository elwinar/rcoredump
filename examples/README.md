# RCoredump Example

This directory containers and example of local configuration that demonstrate
how to setup the server, and generate a demonstration dataset from the crashers
contained in the repository.

0. From the root of the repository,
1. Build both the client, server, and crashers,

		make rcoredump rcoredumpd crashers

2. Start the server,

		./build/rcoredumpd -conf examples/rcoredumpd.conf

3. Configure your host to send coredumps using the client,

		sudo sysctl -w kernel.core_pattern="|/path/to/rcoredump/build/rcoredump -conf /path/to/rcoredump/examples/rcoredump.conf %E %t"
	
	*Note* There may be more commands involved for enable coredumps on your
	system, which goes beyond the goal of this example. A detailed
	explanation on the matter can be found
	[here](https://linux-audit.com/understand-and-configure-core-dumps-work-on-linux/).

4. Run a few crashers to generate stack traces,

		for crasher in $(ls ./build/crashers/*)
		    do ./build/crashers/$crasher
		done

5. Go to http://localhost:1105 to see the results;

