import * as cdk from '@aws-cdk/core';
import * as logs from '@aws-cdk/aws-logs';
import * as cw from "@aws-cdk/aws-cloudwatch";
import * as ecr from '@aws-cdk/aws-ecr';
import * as ecrassets from '@aws-cdk/aws-ecr-assets';
import * as events from '@aws-cdk/aws-events';
import * as iam from '@aws-cdk/aws-iam';
import * as lambda from '@aws-cdk/aws-lambda';
import * as s3 from '@aws-cdk/aws-s3';
import * as s3assets from '@aws-cdk/aws-s3-assets'
import * as targets from '@aws-cdk/aws-events-targets';
import * as sns from '@aws-cdk/aws-sns';
import * as cwactions from "@aws-cdk/aws-cloudwatch-actions";
import {Md5} from 'ts-md5'
import * as path from 'path'

// TODO FIX this later
// import * as handlers from '../../typescript/src/handlers';

interface LambdaExecutionExtras {
  suffix?: string;
}

export class DevopsStack extends cdk.Stack {
  lambdaAlarmTopic: sns.ITopic
  covidScraperHTMLBucket: s3.Bucket
  cloudWatchLogsPolicy: iam.PolicyStatement
  parameterStorePolicy: iam.PolicyStatement
  dashboards: Map<string, cw.Dashboard> = new Map()
  logDashboards: Map<string, cw.Dashboard> = new Map()

  constructor(scope: cdk.App, id: string, props?: cdk.StackProps) {
    super(scope, id, props);
   
    this.lambdaAlarmTopic = sns.Topic.fromTopicArn( this, 'AlarmTopic', 'arn:aws:sns:us-west-2:508192331067:CovidWA-Lambda-All-Alarms')

    // Create S3 bucket for html / possibly screenshots
    const bucketName = 'covidwa-scrapers-html';
    this.covidScraperHTMLBucket = new s3.Bucket(this, bucketName, {
      publicReadAccess: false,
      bucketName,
      // we are going to have a lot of images lets only keep them for a day
      lifecycleRules: [{expiration: cdk.Duration.days(1)}]
    });

    this.createDashboards()

    // CloudWatch Logs Policy
    this.cloudWatchLogsPolicy = new iam.PolicyStatement({
      actions: [
        'logs:CreateLogGroup',
        'logs:CreateLogStream',
        'logs:PutLogEvents',
        'logs:PutMetricFilter',
      ],
      effect: iam.Effect.ALLOW,
    });
    this.cloudWatchLogsPolicy.addAllResources();

    this.parameterStorePolicy = new iam.PolicyStatement({
      actions: [
        'ssm:GetParameter'
      ],
      effect: iam.Effect.ALLOW,
    });
    this.parameterStorePolicy.addAllResources();

    // Push ts docker image to ECR
    const typescriptLambdaAsset = new ecrassets.DockerImageAsset(this, 'typescript-scrapers-asset', {
      directory: path.join(__dirname, '../../typescript'),
    });

    // Push go, python bundles to S3
    const golangLambdaAsset = new s3assets.Asset(this, 'golang-scrapers-asset', {
      path: path.join(__dirname, '../../golang/lambda_stage'),
    });

    const pythonLambdaAsset = new s3assets.Asset(this, 'python-scrapers-asset', {
      path: path.join(__dirname, '../../python/lambda_stage'),
    });
 
    // Deploy from ECR to Lambda
    const tsImageUriParts = typescriptLambdaAsset.imageUri.split(":")
    const tsImageTag = tsImageUriParts[tsImageUriParts.length-1]
     
    // get all the exports in index.ts and create an array
    // TODO FIX THIS LATER
    // const handlersExportName = Object.keys(handlers);
    const highFrequencyTypescriptScraperInfo = [
      this.generateLambdaInfo(['allScrapersHandler'], typescriptLambdaAsset, tsImageTag, 1440),
    ];
    for (const [{lambdaFunction, lambdaProps}] of highFrequencyTypescriptScraperInfo) {
      const {functionName, memorySize, timeout} = lambdaProps;
      this.wireupHighFrequencyLambda(lambdaFunction, functionName!, memorySize, timeout);
    }

    this.generateLambdaInfo([
      'albertsonsHandler1',
      'albertsonsHandler2',
      'albertsonsHandler3',
      'albertsonsHandler4',
    ], typescriptLambdaAsset, tsImageTag, 1024, { suffix:'day' }).map(({lambdaFunction, lambdaProps}) => {
      const {functionName, memorySize, timeout} = lambdaProps;
      return this.wireupAlbertsonsDayLambda(lambdaFunction, functionName!, memorySize, timeout);
    });
    this.generateLambdaInfo([
      'albertsonsHandler1',
      'albertsonsHandler2',
      'albertsonsHandler3',
      'albertsonsHandler4',
    ], typescriptLambdaAsset, tsImageTag, 768, { suffix:'night' }).map(({lambdaFunction, lambdaProps}) => {
      const {functionName, memorySize, timeout} = lambdaProps;
      return this.wireupAlbertsonsCostSavingLambda(lambdaFunction, functionName!, memorySize, timeout);
    });

    this.generateLambdaInfo([
      'riteAidHandler1',
      'riteAidHandler2',
      'riteAidHandler3',
      'riteAidHandler4',
      'riteAidHandler5',
      'riteAidHandler6',
      'riteAidHandler7',
      'riteAidHandler8',
    ], typescriptLambdaAsset, tsImageTag, 1024).map(({lambdaFunction, lambdaProps}) => {
      const {functionName, memorySize, timeout} = lambdaProps;
      return this.wireupHighFrequencyLambda(lambdaFunction, functionName!, memorySize, timeout);
    });

    // Deploy from S3 to Lambda
    const golangLambdaName = 'golang-scrapers';
    const golangLambdaProps:lambda.FunctionProps = {
      runtime: lambda.Runtime.GO_1_X,
      code: lambda.Code.fromBucket(golangLambdaAsset.bucket, golangLambdaAsset.s3ObjectKey),
      functionName: golangLambdaName,
      handler: 'covidwa-scrapers-go-lambda',
      memorySize: 352,
      timeout: cdk.Duration.seconds(300),
    };
    const golangLambdaFunction =  new lambda.Function(this, golangLambdaName, golangLambdaProps);
    this.wireupZipLambda(golangLambdaFunction, golangLambdaProps)

    const pythonLambdaName = 'python-scrapers';
    const pythonLambdaProps: lambda.FunctionProps = {
      runtime: lambda.Runtime.PYTHON_3_8,
      code: lambda.Code.fromBucket(pythonLambdaAsset.bucket, pythonLambdaAsset.s3ObjectKey),
      functionName: pythonLambdaName,
      handler: 'ScrapeAllAndSend.handler',
      memorySize: 160,
      timeout: cdk.Duration.seconds(900),
    };
    const pythonLambdaFunction =  new lambda.Function(this, pythonLambdaName, pythonLambdaProps);
    this.wireupZipLambda(pythonLambdaFunction, pythonLambdaProps)
  }

  generateLambdaInfo(handlerExportNames: string[], typescriptLambdaAsset: ecrassets.DockerImageAsset, tsImageTag: string, memoryLimit: number, extras?: LambdaExecutionExtras) {
    const { suffix } = extras || {};
    return handlerExportNames.map(handler => {
      const handlerName = suffix ? `${handler}-${suffix}` : handler;
      const functionName = `typescript-${handlerName}-scrapers`;
      const lambdaProps: lambda.DockerImageFunctionProps = {
          code:  lambda.DockerImageCode.fromEcr(typescriptLambdaAsset.repository, {
          tag: tsImageTag,
          cmd: [`/app/build/src/handlers/index.${handler}`]
        }),
        functionName,
        memorySize: memoryLimit,
        timeout: cdk.Duration.seconds(900),
      }
      const lambdaFunction = new lambda.DockerImageFunction(this, functionName, lambdaProps);
      return {lambdaFunction, lambdaProps};
    })
  }

  wireupZipLambda(lambdaFn: lambda.IFunction, lambdaProps: lambda.FunctionProps) {
    this.wireupHighFrequencyLambda(lambdaFn, lambdaProps.functionName!, lambdaProps.memorySize, lambdaProps.timeout)
  }

  wireupHighFrequencyLambda(lambdaFn: lambda.IFunction, lambdaName: string, lambdaMaxMemory?: number, lambdaTimeout?: cdk.Duration) {
    const intervalInMinutes = 5;
    //cron start time based on hash
    const hash = Md5.hashStr(lambdaName, true) as Int32Array
    let startMinute = hash[0] % intervalInMinutes;
    if (startMinute < 0) {
      startMinute += intervalInMinutes;
    }
    const schedule = events.Schedule.expression(`cron(${startMinute}/${intervalInMinutes} * ? * * *)`);
    
    this.wireupLambda(lambdaFn, lambdaName, schedule, lambdaMaxMemory, lambdaTimeout);
  }
  
  wireupAlbertsonsDayLambda(lambdaFn: lambda.IFunction, lambdaName: string, lambdaMaxMemory?: number, lambdaTimeout?: cdk.Duration) {
    const intervalInMinutes = 5;
    //cron start time based on hash
    const hash = Md5.hashStr(lambdaName, true) as Int32Array
    let startMinute = hash[0] % intervalInMinutes;
    if (startMinute < 0) {
      startMinute += intervalInMinutes;
    }
    const schedule = events.Schedule.expression(`cron(${startMinute}/${intervalInMinutes} 0-3,15-23 ? * * *)`);
    
    this.wireupLambda(lambdaFn, lambdaName, schedule, lambdaMaxMemory, lambdaTimeout);
  }
  
  wireupAlbertsonsCostSavingLambda(lambdaFn: lambda.IFunction, lambdaName: string, lambdaMaxMemory?: number, lambdaTimeout?: cdk.Duration) {
    const intervalInMinutes = 10;
    //cron start time based on hash
    const hash = Md5.hashStr(lambdaName, true) as Int32Array
    let startMinute = hash[0] % intervalInMinutes;
    if (startMinute < 0) {
      startMinute += intervalInMinutes;
    }
    
    const schedule = events.Schedule.expression(`cron(${startMinute} 4-14 ? * * *)`);
    
    this.wireupLambda(lambdaFn, lambdaName, schedule, lambdaMaxMemory, lambdaTimeout);
  }

  wireupLambda(lambdaFn: lambda.IFunction, lambdaName: string, schedule: events.Schedule, lambdaMaxMemory?: number, lambdaTimeout?: cdk.Duration) {
    /**
    const reportLogFilterPattern = '[f0=REPORT, f1, requestId, f3="Duration:", duration, f5, f6, f7="Duration:", billedDuration, f9=ms, f10=Memory, f11="Size:", allocMemory=*, allocMemoryUnit, f14=Max, f15=Memory, f16="Used:", maxMemory=*, maxMemoryUnit=*]'

    const logGroupName = `/aws/lambda/${lambdaName}`

    const logGroup = new logs.LogGroup(this, logGroupName, {
      logGroupName: logGroupName,
      retention: logs.RetentionDays.ONE_WEEK,
    });

    let treatMissingData = cw.TreatMissingData.NOT_BREACHING;

    if (schedule.expressionString.includes('* ? * * *')) {
      //high frequency lambdas only
      treatMissingData = cw.TreatMissingData.BREACHING;
    }

    //Fatal Errors
    const fatalErrorsAlarm = new cw.Alarm(this, `${lambdaName}FatalErrorsAlarm`, {
      alarmDescription: "Errors >= 1 for 3 datapoints within 20 minutes",
      metric: lambdaFn.metricErrors({ statistic: "sum" }),
      actionsEnabled: true,
      statistic: 'sum',
      threshold: 1,
      evaluationPeriods: 4,
      datapointsToAlarm: 3,
      comparisonOperator: cw.ComparisonOperator.GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
      treatMissingData: treatMissingData,
    });
    fatalErrorsAlarm.addAlarmAction(new cwactions.SnsAction(this.lambdaAlarmTopic));
    fatalErrorsAlarm.addOkAction(new cwactions.SnsAction(this.lambdaAlarmTopic));

    //Short Duration (<100ms = Build Broken)
    const lowDurationAlarm = new cw.Alarm(this, `${lambdaName}LowDurationAlarm`, {
      alarmDescription: "Duration < 100 ms for 1 datapoints within 10 minutes",
      metric: lambdaFn.metricDuration({ statistic: "min" }),
      actionsEnabled: true,
      statistic: 'min',
      threshold: 100,
      evaluationPeriods: 2,
      datapointsToAlarm: 1,
      comparisonOperator: cw.ComparisonOperator.LESS_THAN_THRESHOLD,
      treatMissingData: cw.TreatMissingData.NOT_BREACHING,
    });

    lowDurationAlarm.addAlarmAction(new cwactions.SnsAction(this.lambdaAlarmTopic));
    lowDurationAlarm.addOkAction(new cwactions.SnsAction(this.lambdaAlarmTopic));

    //Long Duration (90% of Timeout)
    let highDurationAlarm = undefined
    if (lambdaTimeout !== undefined) {
      const highDurationThreshold = lambdaTimeout!.toSeconds() * 900 //90% of timeout, s -> ms
      highDurationAlarm = new cw.Alarm(this, `${lambdaName}HighDurationAlarm`, {
        alarmDescription: `Duration > ${highDurationThreshold} ms for 3 datapoints within 20 minutes`,
        metric: lambdaFn.metricDuration({ statistic: "max" }),
        actionsEnabled: true,
        statistic: 'max',
        threshold: highDurationThreshold,
        evaluationPeriods: 4,
        datapointsToAlarm: 3,
        comparisonOperator: cw.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cw.TreatMissingData.NOT_BREACHING,
      });

      highDurationAlarm.addAlarmAction(new cwactions.SnsAction(this.lambdaAlarmTopic));
      highDurationAlarm.addOkAction(new cwactions.SnsAction(this.lambdaAlarmTopic));
    }

    //Memory Usage (93.75% of Max Memory)
    let highMemoryAlarm = undefined
    if (lambdaMaxMemory !== undefined) {
      const memoryMetricFilter = logGroup.addMetricFilter(
        `${lambdaName}-memory`,
        {
          metricNamespace: 'DevopsStack',
          metricName: `${lambdaName}-memory`,
          filterPattern: { logPatternString: reportLogFilterPattern },
          metricValue: '$maxMemory'
        }
      );

      const memoryThreshold = Math.floor(lambdaMaxMemory! * 0.9375) //93.75% (15/16) of hard limit
      highMemoryAlarm = new cw.Alarm(this, `${lambdaName}HighMemoryAlarm`, {
        alarmDescription: `Memory Usage > ${memoryThreshold} MB for 2 datapoints within 10 minutes`,
        metric: memoryMetricFilter.metric({ statistic: "avg" }),
        actionsEnabled: true,
        statistic: 'avg',
        threshold: memoryThreshold,
        evaluationPeriods: 2,
        datapointsToAlarm: 2,
        comparisonOperator: cw.ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: cw.TreatMissingData.NOT_BREACHING,
      });

      highMemoryAlarm.addAlarmAction(new cwactions.SnsAction(this.lambdaAlarmTopic));
      highMemoryAlarm.addOkAction(new cwactions.SnsAction(this.lambdaAlarmTopic));
    }

    lambdaFn.addToRolePolicy(this.cloudWatchLogsPolicy);
    lambdaFn.addToRolePolicy(this.parameterStorePolicy);

    this.covidScraperHTMLBucket.grantReadWrite(lambdaFn);
    
    const rule = new events.Rule(this, `${lambdaName}-rule`, {schedule});
    rule.addTarget(new targets.LambdaFunction(lambdaFn));

    this.addDashboardWidgets(lambdaName, logGroup, fatalErrorsAlarm, highDurationAlarm, highMemoryAlarm);
    **/
  }

  createDashboards(languageNames: string[] = ['Golang', 'Python', 'TypeScript']) {
    /**
    for (const lang of languageNames) {
      const langLowerCase = lang.toLowerCase();
      const dashboardName = `${langLowerCase}-scrapers-auto`;
      const dashboard = new cw.Dashboard(this, dashboardName, {
        dashboardName: dashboardName,
      });
      dashboard.applyRemovalPolicy(cdk.RemovalPolicy.DESTROY);
      this.dashboards.set(langLowerCase, dashboard);

      const logDashboardName = `${langLowerCase}-scrapers-logs`;
      const logDashboard = new cw.Dashboard(this, logDashboardName, {
        dashboardName: logDashboardName,
      });
      dashboard.applyRemovalPolicy(cdk.RemovalPolicy.DESTROY);
      this.logDashboards.set(langLowerCase, logDashboard);
    }
    **/
  }

  addDashboardWidgets(lambdaName: string, logGroup: logs.LogGroup, fatalErrorsAlarm: cw.IAlarm, highDurationAlarm?: cw.IAlarm, highMemoryAlarm?: cw.IAlarm, languageNames: string[] = ['Golang', 'Python', 'TypeScript']) {
    let fnLang = ""
    for (const lang of languageNames) {
      if (lambdaName.toLowerCase().includes(lang.toLowerCase())) {
        fnLang = lang;
        break;
      }
    }

    const fnLangLowerCase = fnLang.toLowerCase();

    if (fnLang === "" || !this.dashboards.has(fnLangLowerCase)) {
      return;
    }    
    const widgetRow: cw.IWidget[] = [];

    const fatalErrorsAlarmWidget = new cw.AlarmWidget({
      alarm: fatalErrorsAlarm,
      title: `Fatal Errors: ${lambdaName}`,
      height: 6,
    });
    widgetRow.push(fatalErrorsAlarmWidget);

    if (highDurationAlarm !== undefined) {
      const highDurationAlarmWidget = new cw.AlarmWidget({
        alarm: highDurationAlarm!,
        title: `Duration: ${lambdaName}`,
        height: 6,
      });
      widgetRow.push(highDurationAlarmWidget);
    }

    if (highMemoryAlarm !== undefined) {
      const highMemoryAlarmWidget = new cw.AlarmWidget({
        alarm: highMemoryAlarm!,
        title: `Memory Used: ${lambdaName}`,
        height: 6,
        leftYAxis: {
          label: "Memory Usage (MB)",
          showUnits: false,
        },
      })
      widgetRow.push(highMemoryAlarmWidget);
    }

    let loggedErrorsFilterPattern = "";
    let loggedErrorsQueryPattern = ""
    if (fnLangLowerCase == "golang") {
      loggedErrorsFilterPattern = '31m ERRO 0m';
      loggedErrorsQueryPattern = 'fields @timestamp, @message | parse @message /.*\\[[0-9m]*\\[(?<type>[^\\[\\]]{4}?)\\][^\\s]*(?<ErrorMessage>.+)/ | filter type="ERRO" | display fromMillis(@timestamp) as Time, ErrorMessage | sort @timestamp desc | limit 50';
    } else if (fnLangLowerCase == "python") {
      loggedErrorsFilterPattern = 'ERROR';
      loggedErrorsQueryPattern = 'fields @timestamp, @message | parse @message /\\[(?<type>\\S+)\\]\\s+(?<Time>\\S+)\\s+(?<id>\\S+)\\s+(?<ErrorMessage>.*)/ | filter type="ERROR" | display Time, ErrorMessage | sort @timestamp desc | limit 50';
    } else {
      loggedErrorsFilterPattern = 'ERROR';
      loggedErrorsQueryPattern = 'fields @timestamp, @message | parse @message /(?<Time>\\S+)\\s+(?<id>\\S+)\\s+(?<type>\\S+)\\s+(2\\S+Z\\s+)?(ERROR:)?\\s*(?<ErrorMessage>.*)/ | filter type="ERROR" | display Time, ErrorMessage | sort @timestamp desc | limit 50';
    }

    const loggedErrorsMetricFilter = logGroup.addMetricFilter(`${lambdaName}-logged-errors`, {
      metricNamespace: 'DevopsStack',
      metricName: `${lambdaName}-logged-errors`,
      filterPattern: { logPatternString: loggedErrorsFilterPattern },
      metricValue: '1',
      defaultValue: 0,
    });

    const loggedErrorsWidget = new cw.GraphWidget({
      left: [loggedErrorsMetricFilter.metric({ statistic: "sum" })],
      leftYAxis: {
        label: "Logged Error Count",
        showUnits: false,
      },
      title: `Logged Errors: ${lambdaName}`,
      height: 6,
    });

    widgetRow.push(loggedErrorsWidget);

    this.dashboards.get(fnLangLowerCase)!.addWidgets(...widgetRow);

    const errorMessagesWidget = new cw.LogQueryWidget({
      logGroupNames: [logGroup.logGroupName],
      queryString: loggedErrorsQueryPattern,
      title: `Logged Error Messages: ${lambdaName}`,
      height: 6,
      width: 24,
    });

    this.dashboards.get(fnLangLowerCase)!.addWidgets(errorMessagesWidget);
  }
}
