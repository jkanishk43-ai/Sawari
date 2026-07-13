import 'package:flutter/material.dart';
import '../../../../core/theme/app_theme.dart';

class ProfileScreen extends StatelessWidget {
  const ProfileScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: SingleChildScrollView(
          child: Column(
            children: [
              const SizedBox(height: 20),
              // Profile Header
              Padding(
                padding: const EdgeInsets.all(20),
                child: Row(
                  children: [
                    CircleAvatar(
                      radius: 40,
                      backgroundColor: AppTheme.primaryColor,
                      child: const Icon(
                        Icons.person,
                        size: 40,
                        color: Colors.white,
                      ),
                    ),
                    const SizedBox(width: 16),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'Guest User',
                            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                                  fontWeight: FontWeight.w600,
                                ),
                          ),
                          const SizedBox(height: 4),
                          Text(
                            'Sign in to sync your data',
                            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                                  color: AppTheme.textSecondary,
                                ),
                          ),
                        ],
                      ),
                    ),
                    IconButton(
                      onPressed: () {},
                      icon: const Icon(Icons.edit_outlined),
                    ),
                  ],
                ),
              ),
              const Divider(),
              // Menu Items
              _MenuItem(
                icon: Icons.phone_android,
                title: 'Phone Number',
                subtitle: '+91 •••• ••• ••••',
                onTap: () {},
              ),
              _MenuItem(
                icon: Icons.location_on_outlined,
                title: 'Saved Places',
                subtitle: 'Home, Work, Other',
                onTap: () {},
              ),
              _MenuItem(
                icon: Icons.payment_outlined,
                title: 'Payment Methods',
                subtitle: 'UPI, Cards',
                onTap: () {},
              ),
              _MenuItem(
                icon: Icons.history,
                title: 'Trip History',
                subtitle: 'View all trips',
                onTap: () {},
              ),
              _MenuItem(
                icon: Icons.local_offer_outlined,
                title: 'Offers & Rewards',
                subtitle: '0 coins available',
                onTap: () {},
              ),
              const Divider(),
              _MenuItem(
                icon: Icons.help_outline,
                title: 'Help & Support',
                onTap: () {},
              ),
              _MenuItem(
                icon: Icons.info_outline,
                title: 'About',
                subtitle: 'Version 1.0.0',
                onTap: () {},
              ),
              const SizedBox(height: 20),
              // Sign In Button
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 20),
                child: SizedBox(
                  width: double.infinity,
                  child: ElevatedButton(
                    onPressed: () {},
                    child: const Text('Sign In'),
                  ),
                ),
              ),
              const SizedBox(height: 40),
            ],
          ),
        ),
      ),
    );
  }
}

class _MenuItem extends StatelessWidget {
  final IconData icon;
  final String title;
  final String? subtitle;
  final VoidCallback onTap;

  const _MenuItem({
    required this.icon,
    required this.title,
    this.subtitle,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: Container(
        padding: const EdgeInsets.all(8),
        decoration: BoxDecoration(
          color: AppTheme.primaryColor.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Icon(
          icon,
          color: AppTheme.primaryColor,
          size: 20,
        ),
      ),
      title: Text(title),
      subtitle: subtitle != null ? Text(subtitle!) : null,
      trailing: const Icon(Icons.chevron_right),
      onTap: onTap,
    );
  }
}
