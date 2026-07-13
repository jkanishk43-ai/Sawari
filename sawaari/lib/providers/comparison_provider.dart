import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/models.dart';
import 'trip_provider.dart';

/// Comparison state for ride options
class ComparisonState {
  final List<RideOption> options;
  final bool isLoading;
  final String? error;
  final SortOption sortBy;

  const ComparisonState({
    this.options = const [],
    this.isLoading = false,
    this.error,
    this.sortBy = SortOption.recommended,
  });

  ComparisonState copyWith({
    List<RideOption>? options,
    bool? isLoading,
    String? error,
    SortOption? sortBy,
  }) {
    return ComparisonState(
      options: options ?? this.options,
      isLoading: isLoading ?? this.isLoading,
      error: error,
      sortBy: sortBy ?? this.sortBy,
    );
  }

  /// Get sorted options based on current sort option
  List<RideOption> get sortedOptions {
    final sorted = List<RideOption>.from(options);
    switch (sortBy) {
      case SortOption.cheapest:
        sorted.sort((a, b) => a.fareMin.compareTo(b.fareMin));
        break;
      case SortOption.fastest:
        sorted.sort((a, b) => a.travelTime.compareTo(b.travelTime));
        break;
      case SortOption.reliability:
        sorted.sort((a, b) => b.reliabilityScore.compareTo(a.reliabilityScore));
        break;
      case SortOption.recommended:
        sorted.sort((a, b) => a.score.compareTo(b.score));
        break;
    }
    return sorted;
  }

  /// Get cheapest option
  RideOption? get cheapestOption {
    if (options.isEmpty) return null;
    return options.reduce((a, b) => a.fareMin < b.fareMin ? a : b);
  }

  /// Get fastest option
  RideOption? get fastestOption {
    if (options.isEmpty) return null;
    return options.reduce((a, b) => a.travelTime < b.travelTime ? a : b);
  }
}

enum SortOption {
  recommended,
  cheapest,
  fastest,
  reliability,
}

/// Comparison notifier
class ComparisonNotifier extends StateNotifier<ComparisonState> {
  ComparisonNotifier() : super(const ComparisonState());

  void setOptions(List<RideOption> options) {
    state = state.copyWith(options: options, isLoading: false, error: null);
  }

  void setLoading(bool loading) {
    state = state.copyWith(isLoading: loading);
  }

  void setError(String error) {
    state = state.copyWith(error: error, isLoading: false);
  }

  void setSortOption(SortOption option) {
    state = state.copyWith(sortBy: option);
  }
}

/// Comparison provider
final comparisonProvider = StateNotifierProvider<ComparisonNotifier, ComparisonState>((ref) {
  return ComparisonNotifier();
});

/// Provider for fetching comparison results
final compareTripsProvider = FutureProvider.family<List<RideOption>, ({double fromLat, double fromLng, double toLat, double toLng})>((ref, params) async {
  final apiClient = ref.read(apiClientProvider);

  try {
    return await apiClient.comparePrices(
      fromLat: params.fromLat,
      fromLng: params.fromLng,
      toLat: params.toLat,
      toLng: params.toLng,
    );
  } catch (e) {
    // Return mock data for demo
    return _getMockRideOptions();
  }
});

/// Mock ride options for demo
List<RideOption> _getMockRideOptions() {
  return [
    const RideOption(
      id: 'bus_1',
      provider: 'DTC',
      name: 'Bus',
      category: RideCategory.bus,
      description: 'DTC / Cluster',
      fareMin: 15,
      fareMax: 25,
      eta: Duration(minutes: 10),
      travelTime: Duration(minutes: 45),
      reliabilityScore: 0.85,
      badge: RideBadge.cheapest,
      fareBreakdown: FareBreakdown(
        baseFare: 5,
        distanceCharge: 10,
        timeCharge: 0,
      ),
    ),
    const RideOption(
      id: 'metro_1',
      provider: 'DMRC',
      name: 'Metro',
      category: RideCategory.metro,
      description: 'DMRC Blue Line',
      fareMin: 30,
      fareMax: 60,
      eta: Duration(minutes: 5),
      travelTime: Duration(minutes: 35),
      reliabilityScore: 0.95,
      badge: RideBadge.fastest,
      fareBreakdown: FareBreakdown(
        baseFare: 11,
        distanceCharge: 19,
        timeCharge: 0,
      ),
    ),
    const RideOption(
      id: 'auto_1',
      provider: 'Meter',
      name: 'Auto',
      category: RideCategory.auto,
      description: 'Meter fare',
      fareMin: 80,
      fareMax: 150,
      eta: Duration(minutes: 3),
      travelTime: Duration(minutes: 30),
      reliabilityScore: 0.70,
      fareBreakdown: FareBreakdown(
        baseFare: 30,
        distanceCharge: 12,
        timeCharge: 2,
      ),
    ),
    const RideOption(
      id: 'uber_go',
      provider: 'Uber',
      name: 'Uber Go',
      category: RideCategory.cab,
      description: 'Affordable, compact rides',
      fareMin: 120,
      fareMax: 200,
      eta: Duration(minutes: 4),
      travelTime: Duration(minutes: 25),
      reliabilityScore: 0.88,
      fareBreakdown: FareBreakdown(
        baseFare: 45,
        distanceCharge: 10,
        timeCharge: 1.5,
      ),
      deeplinkUrl: 'https://m.uber.com/ul/',
    ),
    const RideOption(
      id: 'uber_premier',
      provider: 'Uber',
      name: 'Uber Premier',
      category: RideCategory.cab,
      description: 'Top rated drivers',
      fareMin: 200,
      fareMax: 350,
      eta: Duration(minutes: 6),
      travelTime: Duration(minutes: 25),
      reliabilityScore: 0.90,
      badge: RideBadge.comfort,
      fareBreakdown: FareBreakdown(
        baseFare: 70,
        distanceCharge: 14,
        timeCharge: 2,
      ),
      deeplinkUrl: 'https://m.uber.com/ul/',
    ),
    const RideOption(
      id: 'ola_auto',
      provider: 'Ola',
      name: 'Ola Auto',
      category: RideCategory.auto,
      description: 'Affordable auto rides',
      fareMin: 70,
      fareMax: 130,
      eta: Duration(minutes: 4),
      travelTime: Duration(minutes: 30),
      reliabilityScore: 0.75,
      fareBreakdown: FareBreakdown(
        baseFare: 25,
        distanceCharge: 10,
        timeCharge: 1.5,
      ),
      deeplinkUrl: 'https://www.olacabs.com/',
    ),
    const RideOption(
      id: 'namma_yatri',
      provider: 'Namma Yatri',
      name: 'Namma Yatri Auto',
      category: RideCategory.auto,
      description: 'Zero commission, driver-set fare',
      fareMin: 60,
      fareMax: 120,
      eta: Duration(minutes: 5),
      travelTime: Duration(minutes: 30),
      reliabilityScore: 0.92,
      badge: RideBadge.smartPick,
      fareBreakdown: FareBreakdown(
        baseFare: 25,
        distanceCharge: 10,
        timeCharge: 1.5,
      ),
      bookingRail: 'ondc',
    ),
  ];
}
