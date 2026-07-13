import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/theme/app_theme.dart';

class RideOptionsScreen extends ConsumerWidget {
  final String fromLocation;
  final String toLocation;

  const RideOptionsScreen({
    super.key,
    required this.fromLocation,
    required this.toLocation,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Choose a ride'),
        leading: IconButton(
          onPressed: () => Navigator.pop(context),
          icon: const Icon(Icons.arrow_back),
        ),
      ),
      body: Column(
        children: [
          // Route Info
          Container(
            padding: const EdgeInsets.all(16),
            color: AppTheme.primaryColor.withValues(alpha: 0.05),
            child: Row(
              children: [
                Column(
                  children: [
                    Container(
                      width: 10,
                      height: 10,
                      decoration: BoxDecoration(
                        color: AppTheme.accentGreen,
                        shape: BoxShape.circle,
                      ),
                    ),
                    Container(
                      width: 2,
                      height: 30,
                      color: Colors.grey.shade400,
                    ),
                    Container(
                      width: 10,
                      height: 10,
                      decoration: BoxDecoration(
                        color: AppTheme.accentRed,
                        shape: BoxShape.circle,
                      ),
                    ),
                  ],
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        fromLocation.isNotEmpty ? fromLocation : 'Pickup location',
                        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                              fontWeight: FontWeight.w500,
                            ),
                      ),
                      const SizedBox(height: 20),
                      Text(
                        toLocation.isNotEmpty ? toLocation : 'Destination',
                        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                              fontWeight: FontWeight.w500,
                            ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),

          // Ride Options
          Expanded(
            child: ListView(
              padding: const EdgeInsets.all(16),
              children: [
                // Bus - Cheapest
                _RideCard(
                  name: 'Bus',
                  description: 'DTC / Cluster • 45 min',
                  price: '₹15',
                  badge: 'CHEAPEST',
                  badgeColor: AppTheme.accentGreen,
                  icon: Icons.directions_bus,
                  iconColor: const Color(0xFF3B82F6),
                  eta: '10 min away',
                  onTap: () => _selectRide(context, 'bus', '₹15'),
                ),

                const SizedBox(height: 12),

                // Metro
                _RideCard(
                  name: 'Metro',
                  description: 'DMRC Blue Line • 35 min',
                  price: '₹30',
                  badge: 'FASTEST',
                  badgeColor: const Color(0xFF8B5CF6),
                  icon: Icons.subway,
                  iconColor: const Color(0xFF8B5CF6),
                  eta: '5 min away',
                  onTap: () => _selectRide(context, 'metro', '₹30'),
                ),

                const SizedBox(height: 12),

                // Auto
                _RideCard(
                  name: 'Auto',
                  description: 'Meter fare • 30 min',
                  price: '₹80',
                  badge: null,
                  badgeColor: null,
                  icon: Icons.local_taxi,
                  iconColor: const Color(0xFFF59E0B),
                  eta: '3 min away',
                  onTap: () => _selectRide(context, 'auto', '₹80'),
                ),

                const SizedBox(height: 12),

                // Uber Go
                _RideCard(
                  name: 'Uber Go',
                  description: 'Affordable, compact rides',
                  price: '₹150',
                  badge: null,
                  badgeColor: null,
                  icon: Icons.car_rental,
                  iconColor: Colors.black,
                  eta: '4 min away',
                  onTap: () => _selectRide(context, 'uber_go', '₹150'),
                ),

                const SizedBox(height: 12),

                // Uber Premier
                _RideCard(
                  name: 'Uber Premier',
                  description: 'Top rated drivers',
                  price: '₹250',
                  badge: 'COMFORT',
                  badgeColor: const Color(0xFF3B82F6),
                  icon: Icons.directions_car,
                  iconColor: Colors.black,
                  eta: '6 min away',
                  onTap: () => _selectRide(context, 'uber_premier', '₹250'),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  void _selectRide(BuildContext context, String rideType, String price) {
    Navigator.pushNamed(context, '/confirm-booking');
  }
}

class _RideCard extends StatelessWidget {
  final String name;
  final String description;
  final String price;
  final String? badge;
  final Color? badgeColor;
  final IconData icon;
  final Color iconColor;
  final String eta;
  final VoidCallback onTap;

  const _RideCard({
    required this.name,
    required this.description,
    required this.price,
    this.badge,
    this.badgeColor,
    required this.icon,
    required this.iconColor,
    required this.eta,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: Colors.grey.shade200),
        ),
        child: Row(
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: iconColor.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Icon(
                icon,
                color: iconColor,
                size: 28,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(
                        name,
                        style: Theme.of(context).textTheme.titleMedium?.copyWith(
                              fontWeight: FontWeight.w600,
                            ),
                      ),
                      if (badge != null) ...[
                        const SizedBox(width: 8),
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: badgeColor!.withValues(alpha: 0.1),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            badge!,
                            style: Theme.of(context).textTheme.labelSmall?.copyWith(
                                  color: badgeColor,
                                  fontWeight: FontWeight.w600,
                                  fontSize: 9,
                                ),
                          ),
                        ),
                      ],
                    ],
                  ),
                  const SizedBox(height: 4),
                  Text(
                    description,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: AppTheme.textSecondary,
                        ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    eta,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: AppTheme.accentGreen,
                          fontWeight: FontWeight.w500,
                        ),
                  ),
                ],
              ),
            ),
            Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Text(
                  price,
                  style: Theme.of(context).textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.w700,
                        color: AppTheme.textPrimary,
                      ),
                ),
                const SizedBox(height: 4),
                const Icon(
                  Icons.chevron_right,
                  color: AppTheme.textSecondary,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
