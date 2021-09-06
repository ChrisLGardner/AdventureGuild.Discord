param containerName string = 'adventureguild'
param containerVersion string = 'main'

@secure()
param Token string
param ImageRegistry string = 'halbarad.azurecr.io'
param ImageRegistryUsername string = 'halbarad'

@secure()
param ImageRegistryPassword string
param HoneycombDataset string

@secure()
param HoneycombApiKey string

param VirtualNetwork string
param VirtualNetworkRG string

resource bot_aci 'Microsoft.ContainerInstance/containerGroups@2018-10-01' = {
  name: containerName
  location: 'westeurope'
  dependsOn: [
    networkProfile
  ]
  properties: {
    containers: [
      {
        name: containerName
        properties: {
          image: '${ImageRegistry}/go/${containerName}:${containerVersion}'
          ports: [
            {
              protocol: 'TCP'
              port: 80
            }
          ]
          environmentVariables: [
            {
              name: 'DISCORD_TOKEN'
              value: Token
            }
            {
              name: 'HONEYCOMB_KEY'
              value: HoneycombApiKey
            }
            {
              name: 'HONEYCOMB_DATASET'
              value: HoneycombDataset
            }
            {
              name: 'COSMOSDB_URI'
              value: listConnectionStrings(cosmosdb.id, '2020-04-01').connectionStrings[0].connectionString
            }
          ]
          resources: {
            requests: {
              memoryInGB: '1.5'
              cpu: 1
            }
          }
        }
      }
    ]
    imageRegistryCredentials: [
      {
        server: ImageRegistry
        username: ImageRegistryUsername
        password: ImageRegistryPassword
      }
    ]
    restartPolicy: 'OnFailure'
    ipAddress: {
      ports: [
        {
          protocol: 'TCP'
          port: 80
        }
      ]
      type: 'Private'
    }
    osType: 'Linux'
    networkProfile: {
      id: resourceId('Microsoft.Network/networkProfiles', containerName)
    }
  }
}

resource networkProfile 'Microsoft.Network/networkProfiles@2020-11-01' = {
  name: containerName
  location: resourceGroup().location
  properties: {
    containerNetworkInterfaceConfigurations: [
      {
        name: 'eth0'
        properties: {
          ipConfigurations: [
            {
              name: 'ipconfigprofile1'
              properties: {
                subnet: {
                  id: resourceId(VirtualNetworkRG,'Microsoft.Network/virtualNetworks/subnets', VirtualNetwork,'default')
                }
              }
            }
          ]
        }
      }
    ]
  }
}

resource cosmosdb 'Microsoft.DocumentDb/databaseAccounts@2020-04-01' = {
  kind: 'MongoDB'
  name: containerName
  location: 'westeurope'
  properties: {
    databaseAccountOfferType: 'Standard'
    locations: [
      {
        id: '${containerName}-westeurope'
        failoverPriority: 0
        locationName: 'westeurope'
      }
    ]
    backupPolicy: {
      type: 'Periodic'
      periodicModeProperties: {
        backupIntervalInMinutes: 1440
        backupRetentionIntervalInHours: 48
      }
    }
    isVirtualNetworkFilterEnabled: false
    virtualNetworkRules: []
    ipRules: []
    dependsOn: []
    enableMultipleWriteLocations: false
    capabilities: [
      {
        name: 'EnableMongo'
      }
      {
        name: 'DisableRateLimitingResponses'
      }
    ]
    apiProperties: {
      serverVersion: '4.0'
    }
    enableFreeTier: false
  }
  tags: {
    defaultExperience: 'Azure Cosmos DB for MongoDB API'
  }
}
