import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:geolocator/geolocator.dart';
import '../models/models.dart';

/// Location state
class LocationState {
  final LocationData? currentLocation;
  final bool isLoading;
  final String? error;
  final bool hasPermission;

  const LocationState({
    this.currentLocation,
    this.isLoading = false,
    this.error,
    this.hasPermission = false,
  });

  LocationState copyWith({
    LocationData? currentLocation,
    bool? isLoading,
    String? error,
    bool? hasPermission,
  }) {
    return LocationState(
      currentLocation: currentLocation ?? this.currentLocation,
      isLoading: isLoading ?? this.isLoading,
      error: error,
      hasPermission: hasPermission ?? this.hasPermission,
    );
  }
}

/// Location notifier with GPS/NavIC fusion support
class LocationNotifier extends StateNotifier<LocationState> {
  StreamSubscription<Position>? _positionSubscription;

  LocationNotifier() : super(const LocationState());

  /// Check and request location permissions
  Future<bool> checkAndRequestPermission() async {
    bool serviceEnabled = await Geolocator.isLocationServiceEnabled();
    if (!serviceEnabled) {
      state = state.copyWith(
        error: 'Location services are disabled',
        hasPermission: false,
      );
      return false;
    }

    LocationPermission permission = await Geolocator.checkPermission();
    if (permission == LocationPermission.denied) {
      permission = await Geolocator.requestPermission();
      if (permission == LocationPermission.denied) {
        state = state.copyWith(
          error: 'Location permission denied',
          hasPermission: false,
        );
        return false;
      }
    }

    if (permission == LocationPermission.deniedForever) {
      state = state.copyWith(
        error: 'Location permissions are permanently denied',
        hasPermission: false,
      );
      return false;
    }

    state = state.copyWith(hasPermission: true, error: null);
    return true;
  }

  /// Get current location once
  Future<void> getCurrentLocation() async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final hasPermission = await checkAndRequestPermission();
      if (!hasPermission) {
        state = state.copyWith(isLoading: false);
        return;
      }

      final position = await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(
          accuracy: LocationAccuracy.high,
          timeLimit: Duration(seconds: 15),
        ),
      );

      final locationData = LocationData(
        latitude: position.latitude,
        longitude: position.longitude,
        accuracy: position.accuracy,
        timestamp: position.timestamp,
        source: _getLocationSource(position),
      );

      state = state.copyWith(
        currentLocation: locationData,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Failed to get location: $e',
      );
    }
  }

  /// Start continuous location tracking
  Future<void> startTracking() async {
    state = state.copyWith(isLoading: true);

    final hasPermission = await checkAndRequestPermission();
    if (!hasPermission) {
      state = state.copyWith(isLoading: false);
      return;
    }

    const locationSettings = LocationSettings(
      accuracy: LocationAccuracy.high,
      distanceFilter: 10, // Update every 10 meters
    );

    _positionSubscription = Geolocator.getPositionStream(
      locationSettings: locationSettings,
    ).listen(
      (Position position) {
        final locationData = LocationData(
          latitude: position.latitude,
          longitude: position.longitude,
          accuracy: position.accuracy,
          timestamp: position.timestamp,
          source: _getLocationSource(position),
        );

        state = state.copyWith(
          currentLocation: locationData,
          isLoading: false,
        );
      },
      onError: (e) {
        state = state.copyWith(
          isLoading: false,
          error: 'Location tracking error: $e',
        );
      },
    );
  }

  /// Stop location tracking
  void stopTracking() {
    _positionSubscription?.cancel();
    _positionSubscription = null;
  }

  /// Determine location source based on accuracy
  LocationSource _getLocationSource(Position position) {
    if (position.accuracy > 50) {
      return LocationSource.network;
    } else if (position.accuracy > 15) {
      return LocationSource.gps;
    } else {
      // High accuracy typically indicates NavIC fusion on supported devices
      return LocationSource.navic;
    }
  }

  @override
  void dispose() {
    stopTracking();
    super.dispose();
  }
}

/// Provider for location state
final locationProvider = StateNotifierProvider<LocationNotifier, LocationState>((ref) {
  return LocationNotifier();
});

/// Provider for nearby stops
final nearbyStopsProvider = FutureProvider.family<List<TransitStop>, ({double lat, double lng})>((ref, params) async {
  // TODO: Replace with actual API call
  // For now, return sample data
  return [
    TransitStop(
      id: 'stop_1',
      name: 'Rajiv Chowk Metro',
      latitude: 28.6139,
      longitude: 77.2090,
      type: StopType.metro,
      routes: ['Blue Line', 'Yellow Line'],
      distance: 250,
    ),
    TransitStop(
      id: 'stop_2',
      name: 'Akshardham Metro',
      latitude: 28.6187,
      longitude: 77.2793,
      type: StopType.metro,
      routes: ['Blue Line'],
      distance: 500,
    ),
  ];
});
