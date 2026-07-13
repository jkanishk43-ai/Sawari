import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:latlong2/latlong.dart';
import '../../../../core/theme/app_theme.dart';
import '../../../../providers/providers.dart';
import '../../../../models/models.dart';

class MapScreen extends ConsumerStatefulWidget {
  const MapScreen({super.key});

  @override
  ConsumerState<MapScreen> createState() => _MapScreenState();
}

class _MapScreenState extends ConsumerState<MapScreen> {
  final MapController _mapController = MapController();

  // Delhi center coordinates
  static const LatLng _delhiCenter = LatLng(28.6139, 77.2090);

  LatLng? _currentLocation;
  LatLng? _destination;
  List<TransitStop> _nearbyStops = [];
  List<LatLng> _routePoints = [];
  bool _isLoadingStops = false;

  @override
  void initState() {
    super.initState();
    _currentLocation = _delhiCenter;
    _fetchNearbyStops();
  }

  @override
  void dispose() {
    _mapController.dispose();
    super.dispose();
  }

  Future<void> _fetchNearbyStops() async {
    if (_currentLocation == null) return;

    setState(() => _isLoadingStops = true);

    try {
      final stopsAsync = await ref.read(nearbyStopsProvider((
        lat: _currentLocation!.latitude,
        lng: _currentLocation!.longitude,
      )).future);

      if (mounted) {
        setState(() {
          _nearbyStops = stopsAsync;
          _isLoadingStops = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() => _isLoadingStops = false);
      }
    }
  }

  void _onMapTap(tapPosition, LatLng point) {
    setState(() {
      _destination = point;
      _routePoints = _currentLocation != null && _destination != null
          ? [_currentLocation!, _destination!]
          : [];
    });
  }

  void _centerOnUserLocation() {
    final locationState = ref.read(locationProvider);
    if (locationState.currentLocation != null) {
      setState(() {
        _currentLocation = locationState.currentLocation!.latLng;
      });
      _mapController.move(_currentLocation!, 15);
      _fetchNearbyStops();
    } else {
      ref.read(locationProvider.notifier).getCurrentLocation().then((_) {
        final newLocation = ref.read(locationProvider).currentLocation;
        if (newLocation != null && mounted) {
          setState(() {
            _currentLocation = newLocation.latLng;
          });
          _mapController.move(_currentLocation!, 15);
          _fetchNearbyStops();
        }
      });
    }
  }

  void _navigateToRideOptions() {
    if (_destination != null && _currentLocation != null) {
      // Use the router to navigate
      Navigator.of(context).pushNamed(
        '/ride-options',
        arguments: {
          'from': _currentLocation!,
          'to': _destination!,
        },
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Stack(
        children: [
          // Map
          FlutterMap(
            mapController: _mapController,
            options: MapOptions(
              initialCenter: _currentLocation ?? _delhiCenter,
              initialZoom: 13.0,
              onTap: _onMapTap,
              maxZoom: 19,
              minZoom: 10,
            ),
            children: [
              // OSM Tile Layer
              TileLayer(
                urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
                userAgentPackageName: 'com.sawaari.app',
                maxZoom: 19,
              ),

              // Route polyline
              if (_routePoints.length >= 2)
                PolylineLayer(
                  polylines: [
                    Polyline(
                      points: _routePoints,
                      color: AppTheme.primaryColor,
                      strokeWidth: 4.0,
                      isDotted: false,
                    ),
                  ],
                ),

              // User location marker
              if (_currentLocation != null)
                MarkerLayer(
                  markers: [
                    Marker(
                      point: _currentLocation!,
                      width: 40,
                      height: 40,
                      child: _CurrentLocationMarker(),
                    ),

                    // Destination marker
                    if (_destination != null)
                      Marker(
                        point: _destination!,
                        width: 40,
                        height: 40,
                        child: _DestinationMarker(),
                      ),

                    // Transit stop markers
                    ..._nearbyStops.map((stop) => Marker(
                          point: stop.latLng,
                          width: 36,
                          height: 36,
                          child: _StopMarker(stop: stop),
                        )),
                  ],
                ),
            ],
          ),

          // Loading indicator for stops
          if (_isLoadingStops)
            Positioned(
              top: 100,
              left: 0,
              right: 0,
              child: Center(
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                  decoration: BoxDecoration(
                    color: Colors.white,
                    borderRadius: BorderRadius.circular(20),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withValues(alpha: 0.1),
                        blurRadius: 10,
                      ),
                    ],
                  ),
                  child: const Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      ),
                      SizedBox(width: 8),
                      Text('Finding nearby stops...'),
                    ],
                  ),
                ),
              ),
            ),

          // Top Bar with back and location button
          SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  _TopBarButton(
                    icon: Icons.arrow_back,
                    onPressed: () => Navigator.pop(context),
                  ),
                  const Spacer(),
                  _TopBarButton(
                    icon: Icons.my_location,
                    onPressed: _centerOnUserLocation,
                  ),
                ],
              ),
            ),
          ),

          // Nearby stops list
          if (_nearbyStops.isNotEmpty)
            Positioned(
              top: 80,
              left: 16,
              child: SizedBox(
                height: 40,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  itemCount: _nearbyStops.length.clamp(0, 5),
                  separatorBuilder: (_, __) => const SizedBox(width: 8),
                  itemBuilder: (context, index) {
                    final stop = _nearbyStops[index];
                    return _NearbyStopChip(
                      stop: stop,
                      onTap: () {
                        _mapController.move(stop.latLng, 16);
                      },
                    );
                  },
                ),
              ),
            ),

          // Bottom Search Card
          Positioned(
            bottom: 0,
            left: 0,
            right: 0,
            child: Container(
              padding: const EdgeInsets.all(20),
              decoration: BoxDecoration(
                color: Colors.white,
                borderRadius: const BorderRadius.vertical(top: Radius.circular(24)),
                boxShadow: [
                  BoxShadow(
                    color: Colors.black.withValues(alpha: 0.1),
                    blurRadius: 20,
                    offset: const Offset(0, -5),
                  ),
                ],
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Drag handle
                  Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(
                      color: Colors.grey.shade300,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                  const SizedBox(height: 20),

                  // Location fields
                  Row(
                    children: [
                      // Location indicators column
                      Column(
                        children: [
                          Container(
                            width: 12,
                            height: 12,
                            decoration: BoxDecoration(
                              color: AppTheme.accentGreen,
                              shape: BoxShape.circle,
                            ),
                          ),
                          Container(
                            width: 2,
                            height: 30,
                            color: Colors.grey.shade300,
                          ),
                          Container(
                            width: 12,
                            height: 12,
                            decoration: BoxDecoration(
                              color: AppTheme.accentRed,
                              shape: BoxShape.circle,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(width: 12),

                      // Location text fields
                      Expanded(
                        child: Column(
                          children: [
                            _LocationField(
                              hint: _currentLocation != null
                                  ? 'Lat: ${_currentLocation!.latitude.toStringAsFixed(4)}'
                                  : 'Pickup location',
                              icon: Icons.my_location,
                              onTap: _centerOnUserLocation,
                            ),
                            const SizedBox(height: 8),
                            _LocationField(
                              hint: _destination != null
                                  ? 'Lat: ${_destination!.latitude.toStringAsFixed(4)}'
                                  : 'Tap map to select destination',
                              icon: Icons.location_on,
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),

                  const SizedBox(height: 20),

                  // See Prices button
                  if (_destination != null)
                    SizedBox(
                      width: double.infinity,
                      child: ElevatedButton(
                        onPressed: _navigateToRideOptions,
                        child: const Text('See Prices'),
                      ),
                    ),

                  // Stops count indicator
                  if (_nearbyStops.isNotEmpty && !_isLoadingStops) ...[
                    const SizedBox(height: 12),
                    Text(
                      '${_nearbyStops.length} stops nearby',
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(
                            color: AppTheme.textSecondary,
                          ),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Current location marker widget with pulsing effect
class _CurrentLocationMarker extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: AppTheme.primaryColor,
        shape: BoxShape.circle,
        border: Border.all(color: Colors.white, width: 3),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.3),
            blurRadius: 8,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: const Icon(
        Icons.person,
        color: Colors.white,
        size: 20,
      ),
    );
  }
}

/// Destination marker widget
class _DestinationMarker extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: AppTheme.accentRed,
        shape: BoxShape.circle,
        border: Border.all(color: Colors.white, width: 3),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.3),
            blurRadius: 8,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: const Icon(
        Icons.location_on,
        color: Colors.white,
        size: 20,
      ),
    );
  }
}

/// Transit stop marker widget
class _StopMarker extends StatelessWidget {
  final TransitStop stop;

  const _StopMarker({required this.stop});

  @override
  Widget build(BuildContext context) {
    final (icon, color) = switch (stop.type) {
      StopType.metro => (Icons.subway, const Color(0xFF8B5CF6)),
      StopType.bus => (Icons.directions_bus, const Color(0xFF3B82F6)),
      StopType.auto => (Icons.local_taxi, const Color(0xFFF59E0B)),
      StopType.rickshaw => (Icons.electric_rickshaw, const Color(0xFF10B981)),
    };

    return GestureDetector(
      onTap: () {
        _showStopInfo(context, stop);
      },
      child: Container(
        decoration: BoxDecoration(
          color: color,
          shape: BoxShape.circle,
          border: Border.all(color: Colors.white, width: 2),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withValues(alpha: 0.2),
              blurRadius: 4,
              offset: const Offset(0, 2),
            ),
          ],
        ),
        child: Icon(
          icon,
          color: Colors.white,
          size: 18,
        ),
      ),
    );
  }

  void _showStopInfo(BuildContext context, TransitStop stop) {
    showModalBottomSheet(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (context) => Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              stop.name,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 8),
            if (stop.routes.isNotEmpty)
              Wrap(
                spacing: 8,
                children: stop.routes.map((route) => Chip(
                      label: Text(route),
                      backgroundColor: AppTheme.primaryColor.withValues(alpha: 0.1),
                    )).toList(),
              ),
            if (stop.distance != null) ...[
              const SizedBox(height: 8),
              Text(
                '${stop.distance!.toInt()}m away',
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                      color: AppTheme.textSecondary,
                    ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// Top bar button widget
class _TopBarButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback onPressed;

  const _TopBarButton({
    required this.icon,
    required this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.1),
            blurRadius: 10,
          ),
        ],
      ),
      child: IconButton(
        onPressed: onPressed,
        icon: Icon(icon),
      ),
    );
  }
}

/// Nearby stop chip widget
class _NearbyStopChip extends StatelessWidget {
  final TransitStop stop;
  final VoidCallback onTap;

  const _NearbyStopChip({
    required this.stop,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(20),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withValues(alpha: 0.1),
              blurRadius: 8,
            ),
          ],
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              stop.type == StopType.metro ? Icons.subway : Icons.directions_bus,
              size: 16,
              color: AppTheme.primaryColor,
            ),
            const SizedBox(width: 6),
            Text(
              stop.name,
              style: Theme.of(context).textTheme.bodySmall,
            ),
            if (stop.distance != null) ...[
              const SizedBox(width: 4),
              Text(
                '${stop.distance!.toInt()}m',
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: AppTheme.textSecondary,
                    ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// Location field widget
class _LocationField extends StatelessWidget {
  final String hint;
  final IconData icon;
  final VoidCallback? onTap;

  const _LocationField({
    required this.hint,
    required this.icon,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: Colors.grey.shade100,
          borderRadius: BorderRadius.circular(12),
        ),
        child: Row(
          children: [
            Icon(
              icon,
              size: 20,
              color: AppTheme.textSecondary,
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                hint,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                      color: AppTheme.textSecondary,
                    ),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
