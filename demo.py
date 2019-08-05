from satellite import SatelliteGroup
from controller import Controller, Command

# logging.basicConfig(level=logging.DEBUG)
with SatelliteGroup('typical') as satellites:
    with Controller('python') as c:
        result = c.benchmark(
            trace=True,
            satellites=satellites,
            runtime=10,
            spans_per_second=100)
